// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/daemon/util"
	"k8s.io/kubernetes/pkg/controller/history"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/errors"
	"github.com/SlinkyProject/slurm-operator/internal/resources"
	"github.com/SlinkyProject/slurm-operator/internal/utils"
)

// NodeSetControl implements the control logic for synchronizing NodeSets and their children Pods. It is implemented
// as an interface to allow for extensions that provide different semantics. Currently, there is only one implementation.
type NodeSetControlInterface interface {
	// SyncNodeSet implements the control logic for Pod creation, update, and deletion, and
	// persistent volume creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	SyncNodeSet(ctx context.Context, req reconcile.Request) error
}

// NewDefaultNodeSetControl returns a new instance of the default implementation NodeSetControlInterface that
// implements the documented semantics for NodeSets. podControl is the PodControlInterface used to create, update,
// and delete Pods and to create PersistentVolumeClaims. statusUpdater is the NodeSetStatusUpdaterInterface used
// to update the status of NodeSets. You should use an instance returned from NewRealNodeSetPodControl() for any
// scenario other than testing.
func NewDefaultNodeSetControl(
	client client.Client,
	kubeClient *kubernetes.Clientset,
	eventRecorder record.EventRecorder,
	podControl *NodeSetPodControl,
	statusUpdater NodeSetStatusUpdaterInterface,
	controllerHistory history.Interface,
	slurmClusters *resources.Clusters,
) NodeSetControlInterface {
	return &defaultNodeSetControl{
		Client:            client,
		KubeClient:        kubeClient,
		eventRecorder:     eventRecorder,
		podControl:        podControl,
		statusUpdater:     statusUpdater,
		controllerHistory: controllerHistory,
		slurmClusters:     slurmClusters,
	}
}

type defaultNodeSetControl struct {
	client.Client
	KubeClient        *kubernetes.Clientset
	eventRecorder     record.EventRecorder
	podControl        *NodeSetPodControl
	statusUpdater     NodeSetStatusUpdaterInterface
	controllerHistory history.Interface
	slurmClusters     *resources.Clusters
}

// getNodeSetPods returns nodeset pods owned by the given set.
// This also reconciles ControllerRef by adopting/orphaning.
// Note that returned histories are pointers to objects in the cache.
// If you want to modify one, you need to deep-copy it first.
func (nsc *defaultNodeSetControl) getNodeSetPods(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
) ([]*corev1.Pod, error) {
	selector, err := metav1.LabelSelectorAsSelector(set.Spec.Selector)
	if err != nil {
		return nil, err
	}

	// List all pods to include those that do not match the selector anymore but
	// have a ControllerRef pointing to this controller.
	optsList := &client.ListOptions{
		Namespace:     set.Namespace,
		LabelSelector: labels.Everything(),
	}
	podList := &corev1.PodList{}
	if err := nsc.List(ctx, podList, optsList); err != nil {
		return nil, err
	}
	pods := utils.ReferenceList(podList.Items)

	podControl := kubecontroller.RealPodControl{
		KubeClient: nsc.KubeClient,
		Recorder:   nsc.eventRecorder,
	}

	filter := func(pod *corev1.Pod) bool {
		// Only claim if it matches our NodeSet name schema. Otherwise release/ignore.
		return isPodFromNodeSet(set, pod)
	}

	// Use ControllerRefManager to adopt/orphan as needed.
	cm := kubecontroller.NewPodControllerRefManager(podControl, set, selector, controllerKind, nsc.canAdoptFunc(set))
	return cm.ClaimPods(ctx, pods, filter)
}

// getNodesToNodeSetPods returns a map from nodes to nodeset pods (corresponding to set) created for the nodes.
// This also reconciles ControllerRef by adopting/orphaning.
// Note that returned histories are pointers to objects in the cache.
// If you want to modify one, you need to deep-copy it first.
func (nsc *defaultNodeSetControl) getNodesToNodeSetPods(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
) ([]*corev1.Node, map[*corev1.Node][]*corev1.Pod, error) {
	logger := log.FromContext(ctx)

	optsList := &client.ListOptions{
		LabelSelector: labels.Everything(),
	}
	nodeList := &corev1.NodeList{}
	if err := nsc.List(ctx, nodeList, optsList); err != nil {
		return nil, nil, fmt.Errorf("failed to get list of nodes: %v", err)
	}
	nodes := utils.ReferenceList(nodeList.Items)
	sort.Sort(utils.NodeByWeight(nodes))

	claimedPods, err := nsc.getNodeSetPods(ctx, set)
	if err != nil {
		return nil, nil, err
	}

	// Group Pods by Node name.
	nodeToNodeSetPods := make(map[*corev1.Node][]*corev1.Pod)
	nodeNameToNode := make(map[string]*corev1.Node, 0)
	for _, node := range nodes {
		nodeNameToNode[node.Name] = node
	}
	for _, pod := range claimedPods {
		nodeName, err := util.GetTargetNodeName(pod)
		if err != nil {
			logger.Error(err, "Failed to get target Node name of the NodeSet Pod",
				"NodeSet", klog.KObj(set), "Pod", klog.KObj(pod))
			continue
		}
		if node, ok := nodeNameToNode[nodeName]; ok {
			nodeToNodeSetPods[node] = append(nodeToNodeSetPods[node], pod)
		}
	}
	return nodes, nodeToNodeSetPods, nil
}

// listRevisions returns a array of the ControllerRevisions that represent the revisions of set. If the returned
// error is nil, the returns slice of ControllerRevisions is valid.
func (nsc *defaultNodeSetControl) listRevisions(set *slinkyv1alpha1.NodeSet) ([]*appsv1.ControllerRevision, error) {
	selector, err := metav1.LabelSelectorAsSelector(set.Spec.Selector)
	if err != nil {
		return nil, err
	}
	return nsc.controllerHistory.ListControllerRevisions(set, selector)
}

func (nsc *defaultNodeSetControl) doAdoptOrphanRevisions(
	set *slinkyv1alpha1.NodeSet,
	revisions []*appsv1.ControllerRevision,
) error {
	for i := range revisions {
		adopted, err := nsc.controllerHistory.AdoptControllerRevision(set, controllerKind, revisions[i])
		if err != nil {
			return err
		}
		revisions[i] = adopted
	}
	return nil
}

// If any adoptions are attempted, we should first recheck for deletion with
// an uncached quorum read sometime after listing Pods/ControllerRevisions (see #42639).
func (nsc *defaultNodeSetControl) canAdoptFunc(set *slinkyv1alpha1.NodeSet) func(ctx context.Context) error {
	return kubecontroller.RecheckDeletionTimestamp(func(ctx context.Context) (metav1.Object, error) {
		namespacedName := types.NamespacedName{
			Namespace: set.Namespace,
			Name:      set.Name,
		}
		fresh := &slinkyv1alpha1.NodeSet{}
		if err := nsc.Get(ctx, namespacedName, fresh); err != nil {
			return nil, err
		}
		if fresh.UID != set.UID {
			return nil, fmt.Errorf("original NodeSet(%s) is gone: got UID(%v), wanted UID(%v)",
				klog.KObj(set), fresh.UID, set.UID)
		}
		return fresh, nil
	})
}

// adoptOrphanRevisions adopts any orphaned ControllerRevisions that match set's Selector. If all adoptions are
// successful the returned error is nil.
func (nsc *defaultNodeSetControl) adoptOrphanRevisions(ctx context.Context, set *slinkyv1alpha1.NodeSet) error {
	revisions, err := nsc.listRevisions(set)
	if err != nil {
		return err
	}
	orphanRevisions := make([]*appsv1.ControllerRevision, 0)
	for i := range revisions {
		if metav1.GetControllerOf(revisions[i]) == nil {
			orphanRevisions = append(orphanRevisions, revisions[i])
		}
		// Add the unique label if it iss not already added to the revision.
		// We use the revision name instead of computing hash, so that we do not
		// need to worry about hash collision
		if _, ok := revisions[i].Labels[slinkyv1alpha1.NodeSetRevisionLabel]; !ok {
			toUpdate := revisions[i].DeepCopy()
			toUpdate.Labels[slinkyv1alpha1.NodeSetRevisionLabel] = toUpdate.Name
			if err := nsc.Update(ctx, toUpdate); err != nil {
				return err
			}
		}
	}
	if len(orphanRevisions) > 0 {
		canAdoptErr := nsc.canAdoptFunc(set)(ctx)
		if canAdoptErr != nil {
			return fmt.Errorf("cannot adopt ControllerRevisions: %v", canAdoptErr)
		}
		return nsc.doAdoptOrphanRevisions(set, orphanRevisions)
	}
	return nil
}

// SyncNodeSet executes the core logic loop for a NodeSet reconcile request.
func (nsc *defaultNodeSetControl) SyncNodeSet(
	ctx context.Context,
	req reconcile.Request,
) error {
	logger := log.FromContext(ctx)

	set := &slinkyv1alpha1.NodeSet{}
	if err := nsc.Get(ctx, req.NamespacedName, set); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("NodeSet has been deleted.", "request", req)
			return nil
		}
		return err
	}

	everything := metav1.LabelSelector{}
	if reflect.DeepEqual(set.Spec.Selector, &everything) {
		nsc.eventRecorder.Eventf(set, corev1.EventTypeWarning, SelectingAllReason,
			"This NodeSet is selecting all pods. A non-empty selector is required.")
		return nil
	}

	// Make a copy now to avoid mutation errors.
	set = set.DeepCopy()

	if err := nsc.adoptOrphanRevisions(ctx, set); err != nil {
		return err
	}

	revisions, err := nsc.listRevisions(set)
	if err != nil {
		return err
	}
	history.SortControllerRevisions(revisions)

	currentRevision, updateRevision, collisionCount, err := nsc.getNodeSetRevisions(set, revisions)
	if err != nil {
		return err
	}

	currentHash := getNodeSetRevisionLabel(currentRevision)
	updateHash := getNodeSetRevisionLabel(updateRevision)

	nodes, nodeToNodeSetPods, err := nsc.getNodesToNodeSetPods(ctx, set)
	if err != nil {
		return fmt.Errorf("could not get node to nodeset pod mapping for NodeSet(%s): %v", klog.KObj(set), err)
	}

	if err := nsc.syncSlurm(ctx, set, nodes, nodeToNodeSetPods); err != nil {
		errors := []error{err}
		if err := nsc.syncNodeSetStatus(ctx, set, nodes, nodeToNodeSetPods, collisionCount, currentHash, false); err != nil {
			errors = append(errors, err)
		}
		return utilerrors.NewAggregate(errors)
	}

	if err := nsc.syncNodeSet(ctx, set, nodes, nodeToNodeSetPods, currentHash); err != nil {
		errors := []error{err}
		if err := nsc.syncNodeSetStatus(ctx, set, nodes, nodeToNodeSetPods, collisionCount, currentHash, false); err != nil {
			errors = append(errors, err)
		}
		return utilerrors.NewAggregate(errors)
	}

	// Handle UpdateStrategy
	if !isNodeSetPaused(set) {
		switch set.Spec.UpdateStrategy.Type {
		case slinkyv1alpha1.OnDeleteNodeSetStrategyType:
			// nsc.syncNodeSet() will have handled it
			break
		case slinkyv1alpha1.RollingUpdateNodeSetStrategyType:
			if err := nsc.syncNodeSetRollingUpdate(ctx, set, nodes, nodeToNodeSetPods, updateHash); err != nil {
				errors := []error{err}
				if err := nsc.syncNodeSetStatus(ctx, set, nodes, nodeToNodeSetPods, collisionCount, updateHash, false); err != nil {
					errors = append(errors, err)
				}
				return utilerrors.NewAggregate(errors)
			}
		}
	}

	err = nsc.truncateHistory(ctx, set, revisions, currentRevision, updateRevision)
	if err != nil {
		err = fmt.Errorf("failed to clean up revisions of NodeSet(%s): %v", klog.KObj(set), err)
		errors := []error{err}
		if err := nsc.syncNodeSetStatus(ctx, set, nodes, nodeToNodeSetPods, collisionCount, currentHash, false); err != nil {
			errors = append(errors, err)
		}
		return utilerrors.NewAggregate(errors)
	}

	return nsc.syncNodeSetStatus(ctx, set, nodes, nodeToNodeSetPods, collisionCount, currentHash, true)
}

// processNodeSetPod handles reconciling the NodeSet Pod state.
// Pods will be created, deleted, or updated depending on their current state.
func (nsc *defaultNodeSetControl) processNodeSetPod(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pods []*corev1.Pod,
	i int,
) error {
	logger := log.FromContext(ctx)

	// Note that pods with phase Succeeded will also trigger this event. This is
	// because final pod phase of evicted or otherwise forcibly stopped pods
	// (e.g. terminated on node reboot) is determined by the exit code of the
	// container, not by the reason for pod termination. We should restart the pod
	// regardless of the exit code.
	if utils.IsFailed(pods[i]) || utils.IsSucceeded(pods[i]) || isNodeSetPodDelete(pods[i]) {
		if !utils.IsTerminating(pods[i]) {
			if err := nsc.processCondemned(ctx, set, pods, i); err != nil {
				return err
			}
		}
		// New pod should be generated on the next sync after the current pod is removed from etcd.
		return nil
	}

	// If we find a Pod that has not been created we create the Pod
	if !utils.IsCreated(pods[i]) {
		if utilfeature.DefaultFeatureGate.Enabled(features.StatefulSetAutoDeletePVC) {
			if isStale, err := nsc.podControl.PodClaimIsStale(ctx, set, pods[i]); err != nil {
				return err
			} else if isStale {
				// If a pod has a stale PVC, no more work can be done this round.
				return err
			}
		}
		if err := nsc.podControl.CreateNodeSetPod(ctx, set, pods[i]); err != nil {
			return err
		}
	}

	// If the Pod is in pending state then trigger PVC creation to create missing PVCs
	if utils.IsPending(pods[i]) {
		logger.V(1).Info("NodeSet is triggering PVC creation for pending Pod",
			"NodeSet", klog.KObj(set), "Pod", klog.KObj(pods[i]))
		if err := nsc.podControl.createMissingPersistentVolumeClaims(ctx, set, pods[i]); err != nil {
			return err
		}
	}

	// If we find a Pod that is currently terminating, we must wait until graceful deletion
	// completes before we continue to make progress.
	if utils.IsTerminating(pods[i]) {
		logger.V(1).Info("NodeSet is waiting for Pod to Terminate",
			"NodeSet", klog.KObj(set), "Pod", klog.KObj(pods[i]))
		return nil
	}

	// If we have a Pod that has been created but is not running and ready we can not make progress.
	// We must ensure that all for each Pod, when we create it, all of its predecessors, with respect to its
	// ordinal, are Running and Ready.
	if !utils.IsRunningAndReady(pods[i]) {
		logger.V(1).Info("NodeSet is waiting for Pod to be Running and Ready",
			"NodeSet", klog.KObj(set), "Pod", klog.KObj(pods[i]))
		return nil
	}

	// If we have a Pod that has been created but is not available we can not make progress.
	// We must ensure that all for each Pod, when we create it, all of its predecessors, with respect to its
	// ordinal, are Available.
	if !utils.IsRunningAndAvailable(pods[i], set.Spec.MinReadySeconds) {
		logger.V(1).Info("NodeSet is waiting for Pod to be Available",
			"NodeSet", klog.KObj(set), "Pod", klog.KObj(pods[i]))
		return nil
	}

	// Enforce the NodeSet invariants
	retentionMatch := true
	if utilfeature.DefaultFeatureGate.Enabled(features.StatefulSetAutoDeletePVC) {
		var err error
		retentionMatch, err = nsc.podControl.ClaimsMatchRetentionPolicy(ctx, set, pods[i])
		// An error is expected if the pod is not yet fully updated, and so return is treated as matching.
		if err != nil {
			retentionMatch = true
		}
	}

	stateMatch := true
	if isNodeSetPodCordon(pods[i]) {
		stateMatch = false
	}

	if identityMatches(set, pods[i]) && storageMatches(set, pods[i]) && retentionMatch && stateMatch {
		return nil
	}

	// Make a deep copy so we do not mutate the shared cache
	pod := pods[i].DeepCopy()
	if err := nsc.podControl.UpdateNodeSetPod(ctx, set, pod); err != nil {
		return err
	}

	return nil
}

// processCondemned handles deleting NodeSet Pods.
func (nsc *defaultNodeSetControl) processCondemned(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pods []*corev1.Pod,
	i int,
) error {
	logger := log.FromContext(ctx)

	if utils.IsTerminating(pods[i]) {
		return nil
	}

	logger.V(1).Info("NodeSet Pod is terminating for scale down",
		"NodeSet", klog.KObj(set), "Pod", klog.KObj(pods[i]))

	err := nsc.podControl.DeleteNodeSetPod(ctx, set, pods[i])
	if errors.IsNodeNotDrained(err) {
		// Not a failure case, try again later
		err = nil
		durationStore.Push(utils.KeyFunc(set), 30*time.Second)
	}

	return err
}

// podsShouldBeOnNode figures out the NodeSet pods to be created and deleted on the given node:
//   - nodesNeedingNodeSetPods: the pods need to start on the node
//   - podsToDelete: the Pods need to be deleted on the node
//   - err: unexpected error
func (nsc *defaultNodeSetControl) podsShouldBeOnNode(
	logger klog.Logger,
	node *corev1.Node,
	nodeToNodeSetPods map[*corev1.Node][]*corev1.Pod,
	set *slinkyv1alpha1.NodeSet,
) (nodesNeedingNodeSetPods []*corev1.Node, podsToDelete []*corev1.Pod) {
	shouldRun, shouldContinueRunning := nodeShouldRunNodeSetPod(node, set)
	nodeSetPods, exists := nodeToNodeSetPods[node]

	switch {
	case shouldRun && !exists:
		// If nodeset pod is supposed to be running on node, but is not, create nodeset pod.
		nodesNeedingNodeSetPods = append(nodesNeedingNodeSetPods, node)
	case shouldContinueRunning:
		// If a nodeset pod failed, delete it
		// If there's non-nodeset pods left on this node, we will create it in the next sync loop
		var nodeSetPodsRunning []*corev1.Pod
		replace := false
		for _, pod := range nodeSetPods {
			if utils.IsTerminating(pod) {
				if shouldRun {
					replace = true
				}
				continue
			}
			if utils.IsFailed(pod) {
				// This is a critical place where NS is often fighting with kubelet that rejects pods.
				// We need to avoid hot looping and backoff.
				backoffKey := failedPodsBackoffKey(set, node.Name)

				now := failedPodsBackoff.Clock.Now()
				inBackoff := failedPodsBackoff.IsInBackOffSinceUpdate(backoffKey, now)
				if inBackoff {
					delay := failedPodsBackoff.Get(backoffKey)
					logger.V(1).Info("Deleting failed Pod on Node has been limited by backoff",
						"Pod", klog.KObj(pod), "Node", klog.KObj(node), "delay", delay)
					durationStore.Push(utils.KeyFunc(set), delay)
					continue
				}

				failedPodsBackoff.Next(backoffKey, now)

				logger.V(1).Info("Found failed NodeSet Pod on Node, will try to kill it",
					"Pod", klog.KObj(pod), "Node", klog.KObj(node))
				msg := fmt.Sprintf("Found failed NodeSet Pod(%s) on Node(%s), will try to kill it",
					klog.KObj(pod), klog.KObj(node))
				// Emit an event so that it's discoverable to users.
				nsc.eventRecorder.Eventf(set, corev1.EventTypeWarning, FailedNodeSetPodReason, msg)
				podsToDelete = append(podsToDelete, pod)
			} else {
				nodeSetPodsRunning = append(nodeSetPodsRunning, pod)
			}
		}

		if shouldRun && replace {
			nodesNeedingNodeSetPods = append(nodesNeedingNodeSetPods, node)
		}

		// If there is more than 1 running pod on a node delete all but the oldest
		if len(nodeSetPodsRunning) <= 1 {
			// There are no excess pods to be pruned, and no pods to create
			break
		}

		sort.Sort(utils.PodByCreationTimestampAndPhase(nodeSetPodsRunning))
		for i := 1; i < len(nodeSetPodsRunning); i++ {
			podsToDelete = append(podsToDelete, nodeSetPodsRunning[i])
		}

	case !shouldContinueRunning && exists:
		// If nodeset pod is not supposed to run on node, but it is, delete all nodeset pods on node.
		for _, pod := range nodeSetPods {
			if utils.IsTerminating(pod) {
				continue
			}
			logger.V(1).Info("If NodeSet Pod is not supposed to run on Node, but it is, delete NodeSet Pod on Node.",
				"Node", klog.KObj(node), "Pod", klog.KObj(pod))
			podsToDelete = append(podsToDelete, pod)
		}
	}

	return nodesNeedingNodeSetPods, podsToDelete
}

// syncNodeSetPods deletes given pods and creates new nodeset set pods on the given nodes
// returns slice with errors if any
func (nsc *defaultNodeSetControl) syncNodeSetPods(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	podsToDelete, podsToProcess []*corev1.Pod,
) error {
	createDiff := len(podsToProcess)
	deleteDiff := len(podsToDelete)

	sort.Sort(utils.PodByCost(podsToDelete))
	sort.Sort(utils.PodByCost(podsToProcess))

	processPodFn := func(i int) error {
		return nsc.processNodeSetPod(ctx, set, podsToProcess, i)
	}
	if _, err := utils.SlowStartBatch(createDiff, kubecontroller.SlowStartInitialBatchSize, processPodFn); err != nil {
		return err
	}

	// Fix pod claims, if necessary.
	if utilfeature.DefaultFeatureGate.Enabled(features.StatefulSetAutoDeletePVC) {
		fixPodClaim := func(i int) error {
			if matchPolicy, err := nsc.podControl.ClaimsMatchRetentionPolicy(ctx, set, podsToDelete[i]); err != nil {
				return err
			} else if !matchPolicy {
				if err := nsc.podControl.UpdatePodClaimForRetentionPolicy(ctx, set, podsToDelete[i]); err != nil {
					return err
				}
			}
			return nil
		}
		if _, err := utils.SlowStartBatch(deleteDiff, kubecontroller.SlowStartInitialBatchSize, fixPodClaim); err != nil {
			return err
		}
	}

	processCondemnedFn := func(i int) error {
		return nsc.processCondemned(ctx, set, podsToDelete, i)
	}
	if _, err := utils.SlowStartBatch(deleteDiff, kubecontroller.SlowStartInitialBatchSize, processCondemnedFn); err != nil {
		return err
	}

	return nil
}

// syncNodeSet performs the main calculations for syncNodeSetPods.
func (nsc *defaultNodeSetControl) syncNodeSet(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	nodes []*corev1.Node,
	nodeToNodeSetPods map[*corev1.Node][]*corev1.Pod,
	hash string,
) error {
	logger := log.FromContext(ctx)

	if set.DeletionTimestamp != nil {
		return nil
	}

	// For each node, if the node is running the nodeset pod but is not supposed to, kill the nodeset
	// pod. If the node is supposed to run the nodeset pod, but is not, create the nodeset pod on the node.
	nodesNeedingNodeSetPods := make([]*corev1.Node, 0)
	podsToDelete := make([]*corev1.Pod, 0)
	podsToProcess := make([]*corev1.Pod, 0)
	var nodesDesireScheduled, newPodCount int
	for _, node := range nodes {
		nodesNeedingNodeSetPodsOnNode, podsToDeleteOnNode := nsc.podsShouldBeOnNode(logger, node, nodeToNodeSetPods, set)

		nodesNeedingNodeSetPods = append(nodesNeedingNodeSetPods, nodesNeedingNodeSetPodsOnNode...)
		podsToDelete = append(podsToDelete, podsToDeleteOnNode...)

		if shouldRun, _ := nodeShouldRunNodeSetPod(node, set); shouldRun {
			nodesDesireScheduled++
		}
		if newPod, _, ok := findUpdatedPodsOnNode(set, nodeToNodeSetPods[node], hash); ok && newPod != nil {
			newPodCount++
			podsToProcess = append(podsToProcess, newPod)
		}
	}

	// Remove unscheduled pods assigned to not existing nodes when nodeset pods are scheduled by scheduler.
	// If node does not exist then pods are never scheduled and ca not be deleted by PodGCController.
	podsToDelete = append(podsToDelete, getUnscheduledPodsWithoutNode(nodes, nodeToNodeSetPods)...)

	// Prune down to replicas, otherwise deploy as DaemonSet
	replicas := nodesDesireScheduled
	if set.Spec.Replicas != nil {
		replicas = int(ptr.Deref(set.Spec.Replicas, 0))
		nodesDesireScheduled = len(podsToProcess)
		if len(podsToProcess) > replicas {
			podsToDelete = append(podsToDelete, podsToProcess[replicas:]...)
			podsToProcess = podsToProcess[:replicas]
		}
	}

	// This is the first deploy process.
	if set.Spec.UpdateStrategy.Type == slinkyv1alpha1.RollingUpdateNodeSetStrategyType &&
		set.Spec.UpdateStrategy.RollingUpdate != nil {
		partition := ptr.Deref(set.Spec.UpdateStrategy.RollingUpdate.Partition, 0)
		if set.Spec.UpdateStrategy.RollingUpdate.Partition != nil && partition != 0 {
			// Creates pods on nodes that needing nodeset pod. If progressive annotation is true, the creation will controlled
			// by partition and only some of nodeset pods will be created. Otherwise nodeset pods will be created on every
			// node that need to start a nodeset pod.
			nodesNeedingNodeSetPods = getNodesNeedingPods(
				newPodCount,
				nodesDesireScheduled,
				int(partition),
				isNodeSetCreationProgressively(set),
				nodesNeedingNodeSetPods)
		}
	}

	// Add new pods from nodesNeedingNodeSetPods
	podsToCreate := make([]*corev1.Pod, 0)
	for _, node := range nodesNeedingNodeSetPods {
		if len(podsToProcess)+len(podsToCreate) >= replicas {
			break
		}
		pod := newNodeSetPod(set, node.Name, hash)
		podsToCreate = append(podsToCreate, pod)
	}

	unhealthy := 0
	for _, pod := range podsToProcess {
		if !utils.IsHealthy(pod) || isNodeSetPodDelete(pod) {
			unhealthy++
		}
	}
	if unhealthy > 0 {
		logger.Info("NodeSet has unhealthy Pods",
			"NodeSet", klog.KObj(set),
			"unhealthyReplicas", unhealthy)
	}

	podsToProcess = append(podsToProcess, podsToCreate...)

	return nsc.syncNodeSetPods(ctx, set, podsToDelete, podsToProcess)
}

var _ NodeSetControlInterface = &defaultNodeSetControl{}
