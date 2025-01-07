// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2017 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/controller/daemon/util"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/SlinkyProject/slurm-client/pkg/object"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/annotations"
	"github.com/SlinkyProject/slurm-operator/internal/utils"
)

// updatedDesiredNodeCounts calculates the true number of allowed unavailable or surge pods and
// updates the nodeToNodeSetPods array to include an empty array for every node that is not scheduled.
func (nsc *defaultNodeSetControl) updatedDesiredNodeCounts(
	logger klog.Logger,
	set *slinkyv1alpha1.NodeSet,
	nodes []*corev1.Node,
	nodeToNodeSetPods map[*corev1.Node][]*corev1.Pod,
) (int, int, error) {
	var desiredNumberScheduled int
	for i := range nodes {
		node := nodes[i]
		wantToRun, _ := nodeShouldRunNodeSetPod(node, set)
		if !wantToRun {
			continue
		}
		desiredNumberScheduled++

		if _, exists := nodeToNodeSetPods[node]; !exists {
			nodeToNodeSetPods[node] = nil
		}
	}

	if set.Spec.Replicas != nil {
		desiredNumberScheduled = int(ptr.Deref(set.Spec.Replicas, 0))
	}

	maxUnavailable, err := unavailableCount(set, desiredNumberScheduled)
	if err != nil {
		return -1, -1, fmt.Errorf("invalid value for MaxUnavailable: %v", err)
	}

	// if the daemonset returned with an impossible configuration, obey the default of unavailable=1 (in the
	// event the apiserver returns 0 for both surge and unavailability)
	if desiredNumberScheduled > 0 && maxUnavailable == 0 {
		logger.Info("NodeSet is not configured for unavailability, defaulting to accepting unavailability",
			"NodeSet", klog.KObj(set))
		maxUnavailable = 1
	}
	return maxUnavailable, desiredNumberScheduled, nil
}

func GetTemplateGeneration(set *slinkyv1alpha1.NodeSet) (*int64, error) {
	annotation, found := set.Annotations[appsv1.DeprecatedTemplateGeneration]
	if !found {
		return nil, nil
	}
	generation, err := strconv.ParseInt(annotation, 10, 64)
	if err != nil {
		return nil, err
	}
	return &generation, nil
}

func (nsc *defaultNodeSetControl) filterNodeSetPodsToUpdate(
	set *slinkyv1alpha1.NodeSet,
	nodes []*corev1.Node,
	hash string,
	nodeToNodeSetPods map[*corev1.Node][]*corev1.Pod,
) (map[*corev1.Node][]*corev1.Pod, error) {
	existingNodes := sets.NewString()
	for _, node := range nodes {
		existingNodes.Insert(node.Name)
	}
	for node := range nodeToNodeSetPods {
		if !existingNodes.Has(node.Name) {
			delete(nodeToNodeSetPods, node)
		}
	}

	nodeNames, err := nsc.filterNodeSetPodsNodeToUpdate(set, hash, nodeToNodeSetPods)
	if err != nil {
		return nil, err
	}

	ret := make(map[*corev1.Node][]*corev1.Pod, len(nodeNames))
	for _, name := range nodeNames {
		ret[name] = nodeToNodeSetPods[name]
	}
	return ret, nil
}

func (nsc *defaultNodeSetControl) filterNodeSetPodsNodeToUpdate(
	set *slinkyv1alpha1.NodeSet,
	hash string,
	nodeToNodeSetPods map[*corev1.Node][]*corev1.Pod,
) ([]*corev1.Node, error) {
	var partition int32
	if set.Spec.UpdateStrategy.RollingUpdate != nil {
		partition = ptr.Deref(set.Spec.UpdateStrategy.RollingUpdate.Partition, 0)
	}
	var allNodes []*corev1.Node
	for node := range nodeToNodeSetPods {
		allNodes = append(allNodes, node)
	}
	sort.Sort(utils.NodeByWeight(allNodes))

	var updated []*corev1.Node
	var updating []*corev1.Node
	var rest []*corev1.Node
	for node := range nodeToNodeSetPods {
		newPod, oldPod, ok := findUpdatedPodsOnNode(set, nodeToNodeSetPods[node], hash)
		if !ok || newPod != nil || oldPod != nil {
			updated = append(updated, node)
			continue
		}
		rest = append(rest, node)
	}

	sorted := append(updated, updating...)
	sorted = append(sorted, rest...)
	if maxUpdate := len(allNodes) - int(partition); maxUpdate <= 0 {
		return nil, nil
	} else if maxUpdate < len(sorted) {
		sorted = sorted[:maxUpdate]
	}
	return sorted, nil
}

// syncNodeSetRollingUpdate identifies the set of old pods to in-place update, delete, or additional pods to create on nodes,
// remaining within the constraints imposed by the update strategy.
func (nsc *defaultNodeSetControl) syncNodeSetRollingUpdate(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	nodes []*corev1.Node,
	nodeToNodeSetPods map[*corev1.Node][]*corev1.Pod,
	hash string,
) error {
	logger := log.FromContext(ctx)

	maxUnavailable, _, err := nsc.updatedDesiredNodeCounts(logger, set, nodes, nodeToNodeSetPods)
	if err != nil {
		return fmt.Errorf("could not get unavailable numbers: %v", err)
	}

	// Advanced: filter the pods updated, updating and can update, according to partition and selector
	nodeToNodeSetPods, err = nsc.filterNodeSetPodsToUpdate(set, nodes, hash, nodeToNodeSetPods)
	if err != nil {
		return fmt.Errorf("failed to filterNodeSetPodsToUpdate: %v", err)
	}

	now := failedPodsBackoff.Clock.Now()

	// We delete just enough pods to stay under the maxUnavailable limit, if any
	// are necessary, and let syncNodeSet create new instances on those nodes.
	//
	// Assumptions:
	// * Expect syncNodeSet to allow no more than one pod per node
	// * Expect syncNodeSet will create new pods
	// * Expect syncNodeSet will handle failed pods
	// * Deleted pods do not count as unavailable so that updates make progress when nodes are down
	// Invariants:
	// * The number of new pods that are unavailable must be less than maxUnavailable
	// * A node with an available old pod is a candidate for deletion if it does not violate other invariants
	//
	var numUnavailable int
	var allowedReplacementPods []*corev1.Pod
	var candidatePodsToDelete []*corev1.Pod
	for node, pods := range nodeToNodeSetPods {
		newPod, oldPod, ok := findUpdatedPodsOnNode(set, pods, hash)
		if !ok {
			// let the syncNodeSet clean up this node, and treat it as an unavailable node
			logger.V(1).Info("NodeSet has excess pods on Node, skipping to allow the core loop to process",
				"NodeSet", klog.KObj(set), "Node", klog.KObj(node))
			numUnavailable++
			continue
		}
		switch {
		case oldPod == nil && newPod == nil, oldPod != nil && newPod != nil:
			// syncNodeSet will handle creating or deleting the appropriate pod
		case newPod != nil:
			// this pod is up to date, check its availability
			if !podutil.IsPodAvailable(newPod, set.Spec.MinReadySeconds, metav1.Time{Time: now}) {
				// an unavailable new pod is counted against maxUnavailable
				numUnavailable++
				logger.V(1).Info("NodeSet Pod on Node is new and unavailable",
					"NodeSet", klog.KObj(set), "Pod", klog.KObj(newPod), "Node", klog.KObj(node))
			}
		default:
			// this pod is old, it is an update candidate
			switch {
			case !podutil.IsPodAvailable(oldPod, set.Spec.MinReadySeconds, metav1.Time{Time: now}), isNodeSetPodDelete(oldPod):
				// the old pod is not available, so it needs to be replaced
				logger.V(1).Info("NodeSet Pod on Node is out of date and not available, allowing replacement",
					"NodeSet", klog.KObj(set), "Pod", klog.KObj(oldPod), "Node", klog.KObj(node))
				// record the replacement
				if allowedReplacementPods == nil {
					allowedReplacementPods = make([]*corev1.Pod, 0, len(nodeToNodeSetPods))
				}
				allowedReplacementPods = append(allowedReplacementPods, oldPod)
			case numUnavailable >= maxUnavailable:
				// no point considering any other candidates
				continue
			default:
				logger.V(1).Info("NodeSet Pod on Node is out of date, this is a candidate to replace",
					"NodeSet", klog.KObj(set), "Pod", klog.KObj(oldPod), "Node", klog.KObj(node))
				// record the candidate
				if candidatePodsToDelete == nil {
					candidatePodsToDelete = make([]*corev1.Pod, 0, maxUnavailable)
				}
				candidatePodsToDelete = append(candidatePodsToDelete, oldPod)
			}
		}
	}

	// use any of the candidates we can, including the allowedReplacementPods
	logger.V(1).Info("NodeSet allowing replacements",
		"NodeSet", klog.KObj(set),
		"allowedReplacementPods", len(allowedReplacementPods),
		"maxUnavailable", maxUnavailable,
		"numUnavailable", numUnavailable,
		"candidatePodsToDelete", len(candidatePodsToDelete))
	remainingUnavailable := maxUnavailable - numUnavailable
	if remainingUnavailable < 0 {
		remainingUnavailable = 0
	}
	if max := len(candidatePodsToDelete); remainingUnavailable > max {
		remainingUnavailable = max
	}
	oldPodsToDelete := append(allowedReplacementPods, candidatePodsToDelete[:remainingUnavailable]...)

	return nsc.syncNodeSetPods(ctx, set, oldPodsToDelete, nil)
}

// updateSlurmNodeWithPodInfo updated the corresponding Slurm node with info of
// the Pod that backs it.
func (nsc *defaultNodeSetControl) updateSlurmNodeWithPodInfo(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	namespacedName := types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Name,
	}
	freshPod := &corev1.Pod{}
	if err := nsc.Get(ctx, namespacedName, freshPod); err != nil {
		return err
	}

	clusterName := types.NamespacedName{
		Namespace: set.GetNamespace(),
		Name:      set.Spec.ClusterName,
	}
	slurmClient := nsc.slurmClusters.Get(clusterName)
	if slurmClient != nil && !isNodeSetPodDelete(pod) {
		objectKey := object.ObjectKey(pod.Spec.Hostname)
		slurmNode := &slurmtypes.Node{}
		if err := slurmClient.Get(ctx, objectKey, slurmNode); err != nil {
			return err
		}

		oldNodeInfo := slurmtypes.NodeInfo{}
		_ = slurmtypes.NodeInfoParse(slurmNode.Comment, &oldNodeInfo)
		nodeInfo := slurmtypes.NodeInfo{
			Namespace: pod.Namespace,
			PodName:   pod.Name,
		}

		if oldNodeInfo.Equal(nodeInfo) {
			// Avoid needless update request
			return nil
		}

		logger.Info("Update Slurm Node with Kubernetes Pod info",
			"Node", slurmNode.Name, "NodeInfo", nodeInfo)

		slurmNode.Comment = nodeInfo.ToString()
		if err := slurmClient.Update(ctx, slurmNode); err != nil {
			return err
		}
	}

	return nil
}

func tolerateError(err error) bool {
	if err == nil {
		return true
	}
	errText := err.Error()
	if errText == http.StatusText(http.StatusNotFound) ||
		errText == http.StatusText(http.StatusNoContent) {
		return true
	}
	return false
}

// syncSlurm processes Slurm Nodes to align them with Kubernetes, and vice versa.
func (nsc *defaultNodeSetControl) syncSlurm(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	nodes []*corev1.Node,
	nodeToNodeSetPods map[*corev1.Node][]*corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	clusterName := types.NamespacedName{
		Namespace: set.GetNamespace(),
		Name:      set.Spec.ClusterName,
	}
	slurmClient := nsc.slurmClusters.Get(clusterName)
	if slurmClient == nil {
		return nil
	}

	nodeList := &slurmtypes.NodeList{}
	if err := slurmClient.List(ctx, nodeList); !tolerateError(err) {
		return err
	}

	kubeNodes := sets.NewString()
	for _, node := range nodes {
		nodeSetPods, exists := nodeToNodeSetPods[node]
		if !exists {
			continue
		}
		kubeNodes.Insert(node.Name)
		for _, pod := range nodeSetPods {
			if !utils.IsHealthy(pod) {
				continue
			}
			if err := nsc.updateSlurmNodeWithPodInfo(ctx, set, pod); !tolerateError(err) {
				return err
			}
		}
	}

	slurmNodes := sets.NewString()
	for _, node := range nodeList.Items {
		hasCommunicationFailure := node.State.HasAll(slurmtypes.NodeStateDOWN, slurmtypes.NodeStateNOTRESPONDING)
		nodeInfo := slurmtypes.NodeInfo{}
		_ = slurmtypes.NodeInfoParse(node.Comment, &nodeInfo)
		noPodInfo := nodeInfo.Equal(slurmtypes.NodeInfo{})
		if kubeNodes.Has(node.Name) || !hasCommunicationFailure || noPodInfo {
			slurmNodes.Insert(getSafeNodeName(node.Name))
			continue
		}
		logger.Info("Deleting Slurm Node without a corresponding Pod", "Node", node.Name, "Pod", node.Comment)
		if err := slurmClient.Delete(ctx, &node); !tolerateError(err) {
			return err
		}
	}

	for _, node := range nodes {
		nodeSetPods, exists := nodeToNodeSetPods[node]
		if !exists {
			continue
		}
		for _, pod := range nodeSetPods {
			if slurmNodes.Has(pod.Spec.Hostname) || !utils.IsHealthy(pod) || !utils.IsRunningAndAvailable(pod, 30) || isNodeSetPodDelete(pod) {
				continue
			}
			toUpdate := pod.DeepCopy()
			toUpdate.Annotations[annotations.PodDelete] = "true"
			if err := nsc.Update(ctx, toUpdate); err != nil {
				if apierrors.IsNotFound(err) {
					return err
				}
			}
		}
	}

	return nil
}

// inconsistentStatus returns true if the ObservedGeneration of status is greater than set's
// Generation or if any of the status's fields do not match those of set's status.
func inconsistentStatus(set *slinkyv1alpha1.NodeSet, status *slinkyv1alpha1.NodeSetStatus) bool {
	return status.ObservedGeneration > set.Status.ObservedGeneration ||
		status.DesiredNumberScheduled != set.Status.DesiredNumberScheduled ||
		status.CurrentNumberScheduled != set.Status.CurrentNumberScheduled ||
		status.NumberMisscheduled != set.Status.NumberMisscheduled ||
		status.NumberReady != set.Status.NumberReady ||
		status.UpdatedNumberScheduled != set.Status.UpdatedNumberScheduled ||
		status.NumberAvailable != set.Status.NumberAvailable ||
		status.NumberUnavailable != set.Status.NumberUnavailable ||
		status.NumberIdle != set.Status.NumberIdle ||
		status.NumberAllocated != set.Status.NumberAllocated ||
		status.NumberDrain != set.Status.NumberDrain ||
		status.NodeSetHash != set.Status.NodeSetHash
}

func (nsc *defaultNodeSetControl) updateStatus(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	status *slinkyv1alpha1.NodeSetStatus,
) error {
	logger := log.FromContext(ctx)

	// do not perform an update when the status is consistant
	if !inconsistentStatus(set, status) {
		return nil
	}

	logger.V(1).Info("NodeSet status update", "NodeSetStatus", status)

	// copy set and update its status
	set = set.DeepCopy()
	if err := nsc.statusUpdater.UpdateNodeSetStatus(ctx, set, status); err != nil {
		return err
	}

	return nil
}

func (nsc *defaultNodeSetControl) syncNodeSetStatus(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	nodes []*corev1.Node,
	nodeToNodeSetPods map[*corev1.Node][]*corev1.Pod,
	collisionCount int32,
	hash string,
	updateObservedGen bool,
) error {
	setKey := utils.KeyFunc(set)
	status := set.Status.DeepCopy()

	clusterName := types.NamespacedName{
		Namespace: set.GetNamespace(),
		Name:      set.Spec.ClusterName,
	}
	slurmClient := nsc.slurmClusters.Get(clusterName)

	selector, err := metav1.LabelSelectorAsSelector(set.Spec.Selector)
	if err != nil {
		return fmt.Errorf("could not get label selector for NodeSet(%s): %v", klog.KObj(set), err)
	}

	var numberIdle, numberAllocated, numberDown, numberDrain int32
	var desiredNumberScheduled, currentNumberScheduled, numberMisscheduled, numberReady, updatedNumberScheduled, numberAvailable int32
	now := failedPodsBackoff.Clock.Now()
	for _, node := range nodes {
		shouldRun, _ := nodeShouldRunNodeSetPod(node, set)
		scheduled := len(nodeToNodeSetPods[node]) > 0

		if shouldRun {
			desiredNumberScheduled++
			if scheduled {
				currentNumberScheduled++
				// Sort the nodeset pods by creation time, so that the oldest is first.
				nodeSetPods := nodeToNodeSetPods[node]
				sort.Sort(utils.PodByCreationTimestampAndPhase(nodeSetPods))
				pod := nodeSetPods[0]
				if podutil.IsPodReady(pod) {
					numberReady++
					if isNodeSetPodAvailable(pod, set.Spec.MinReadySeconds, metav1.Time{Time: now}) {
						numberAvailable++
					}
				}
				// If the returned error is not nil we have a parse error.
				// The controller handles this via the hash.
				generation, err := GetTemplateGeneration(set)
				if err != nil {
					generation = nil
				}
				if util.IsPodUpdated(pod, hash, generation) {
					updatedNumberScheduled++
				}
			}
		} else {
			if scheduled {
				numberMisscheduled++
			}
		}

		if slurmClient != nil {
			slurmNode := &slurmtypes.Node{}
			key := object.ObjectKey(node.Name)
			if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
				if err.Error() != http.StatusText(http.StatusNotFound) {
					return fmt.Errorf("failed to get Slurm Node: %v", err)
				}
			}

			nodeInfo := slurmtypes.NodeInfo{}
			_ = slurmtypes.NodeInfoParse(slurmNode.Comment, &nodeInfo)

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: nodeInfo.Namespace,
					Name:      nodeInfo.PodName,
				},
			}
			if !isPodFromNodeSet(set, pod) {
				continue
			}

			if utils.IsHealthy(pod) {
				if err := nsc.updateSlurmNodeWithPodInfo(ctx, set, pod); err != nil {
					if err.Error() != http.StatusText(http.StatusNotFound) {
						return err
					}
				}
			}

			// Base Slurm Node States
			switch {
			case slurmNode.State.Has(slurmtypes.NodeStateIDLE):
				numberIdle++
			case slurmNode.State.HasAny(slurmtypes.NodeStateALLOCATED, slurmtypes.NodeStateMIXED):
				numberAllocated++
			case slurmNode.State.Has(slurmtypes.NodeStateDOWN):
				numberDown++
			}
			// Flag Slurm Node State
			if slurmNode.State.Has(slurmtypes.NodeStateDRAIN) {
				numberDrain++
			}
		}
	}
	if set.Spec.Replicas != nil {
		desiredNumberScheduled = ptr.Deref(set.Spec.Replicas, 0)
	}
	numberUnavailable := desiredNumberScheduled - numberAvailable

	if updateObservedGen {
		status.ObservedGeneration = set.Generation
	}
	status.DesiredNumberScheduled = desiredNumberScheduled
	status.CurrentNumberScheduled = currentNumberScheduled
	status.NumberMisscheduled = numberMisscheduled
	status.NumberReady = numberReady
	status.UpdatedNumberScheduled = updatedNumberScheduled
	status.NumberAvailable = numberAvailable
	status.NumberUnavailable = numberUnavailable
	status.NumberIdle = numberIdle
	status.NumberAllocated = numberAllocated
	status.NumberDown = numberDown
	status.NumberDrain = numberDrain
	status.NodeSetHash = hash
	status.CollisionCount = &collisionCount
	status.Selector = selector.String()

	if err := nsc.updateStatus(ctx, set, status); err != nil {
		return fmt.Errorf("error updating NodeSet(%s) status: %v", setKey, err)
	}

	if set.Spec.MinReadySeconds >= 0 && numberReady != numberAvailable {
		// Resync the NodeSet after MinReadySeconds as a last line of defense to guard against clock-skew.
		durationStore.Push(setKey, time.Duration(set.Spec.MinReadySeconds)*time.Second)
	} else if (numberIdle + numberAllocated) != desiredNumberScheduled {
		// Resync the NodeSet until the Slurm state is correct
		durationStore.Push(setKey, 5*time.Second)
	}

	return nil
}
