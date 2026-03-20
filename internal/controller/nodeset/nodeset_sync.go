// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	v1helper "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	daemonutils "k8s.io/kubernetes/pkg/controller/daemon/util"
	"k8s.io/kubernetes/pkg/controller/history"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/util/taints"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/SlinkyProject/slurm-operator/internal/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/historycontrol"
	"github.com/SlinkyProject/slurm-operator/internal/utils/mathutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podcontrol"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
	slurmtaints "github.com/SlinkyProject/slurm-operator/pkg/taints"
)

const (
	burstReplicas = 250

	// FailedDaemonPodReason is added to an event when the status of a Pod of a DaemonSet is 'Failed'.
	FailedDaemonPodReason = "FailedDaemonPod"
	// SucceededDaemonPodReason is added to an event when the status of a Pod of a DaemonSet is 'Succeeded'.
	SucceededDaemonPodReason = "SucceededDaemonPod"
)

// Sync implements control logic for synchronizing a NodeSet and its derived Pods.
func (r *NodeSetReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	nodeset := &slinkyv1beta1.NodeSet{}
	if err := r.Get(ctx, req.NamespacedName, nodeset); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(3).Info("NodeSet has been deleted.", "request", req)
			r.expectations.DeleteExpectations(logger, req.String())
			return nil
		}
		return err
	}

	// Make a copy now to avoid client cache mutation.
	nodeset = nodeset.DeepCopy()
	defaults.SetNodeSetDefaults(nodeset)
	key := objectutils.KeyFunc(nodeset)

	if nodeset.DeletionTimestamp.IsZero() {
		durationStore.Push(key, 30*time.Second)
	}

	if err := r.adoptOrphanRevisions(ctx, nodeset); err != nil {
		return err
	}

	revisions, err := r.listRevisions(nodeset)
	if err != nil {
		return err
	}

	currentRevision, updateRevision, collisionCount, err := r.getNodeSetRevisions(nodeset, revisions)
	if err != nil {
		return err
	}
	hash := historycontrol.GetRevision(updateRevision.GetLabels())

	nodesetPods, err := r.getNodeSetPods(ctx, nodeset)
	if err != nil {
		return err
	}

	if !r.expectations.SatisfiedExpectations(logger, key) || nodeset.DeletionTimestamp != nil {
		return r.syncStatus(ctx, nodeset, nodesetPods, currentRevision, updateRevision, collisionCount, hash)
	}

	if err := r.sync(ctx, nodeset, nodesetPods, hash); err != nil {
		return r.syncStatus(ctx, nodeset, nodesetPods, currentRevision, updateRevision, collisionCount, hash, err)
	}

	if r.expectations.SatisfiedExpectations(logger, key) {
		if err := r.syncUpdate(ctx, nodeset, nodesetPods, hash); err != nil {
			return r.syncStatus(ctx, nodeset, nodesetPods, currentRevision, updateRevision, collisionCount, hash, err)
		}
		if err := r.truncateHistory(ctx, nodeset, revisions, currentRevision, updateRevision); err != nil {
			err = fmt.Errorf("failed to clean up revisions of NodeSet(%s): %w", klog.KObj(nodeset), err)
			return r.syncStatus(ctx, nodeset, nodesetPods, currentRevision, updateRevision, collisionCount, hash, err)
		}
	}

	return r.syncStatus(ctx, nodeset, nodesetPods, currentRevision, updateRevision, collisionCount, hash)
}

// adoptOrphanRevisions adopts any orphaned ControllerRevisions that match nodeset's Selector. If all adoptions are
// successful the returned error is nil.
func (r *NodeSetReconciler) adoptOrphanRevisions(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) error {
	revisions, err := r.listRevisions(nodeset)
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
		if _, ok := revisions[i].Labels[history.ControllerRevisionHashLabel]; !ok {
			toUpdate := revisions[i].DeepCopy()
			toUpdate.Labels[history.ControllerRevisionHashLabel] = toUpdate.Name
			if err := r.Update(ctx, toUpdate); err != nil {
				return err
			}
		}
	}
	if len(orphanRevisions) > 0 {
		canAdoptErr := r.canAdoptFunc(nodeset)(ctx)
		if canAdoptErr != nil {
			return fmt.Errorf("cannot adopt ControllerRevisions: %w", canAdoptErr)
		}
		return r.doAdoptOrphanRevisions(nodeset, orphanRevisions)
	}
	return nil
}

func (r *NodeSetReconciler) doAdoptOrphanRevisions(
	nodeset *slinkyv1beta1.NodeSet,
	revisions []*appsv1.ControllerRevision,
) error {
	for i := range revisions {
		adopted, err := r.historyControl.AdoptControllerRevision(nodeset, slinkyv1beta1.NodeSetGVK, revisions[i])
		if err != nil {
			return err
		}
		revisions[i] = adopted
	}
	return nil
}

// listRevisions returns a array of the ControllerRevisions that represent the revisions of nodeset. If the returned
// error is nil, the returns slice of ControllerRevisions is valid.
func (r *NodeSetReconciler) listRevisions(nodeset *slinkyv1beta1.NodeSet) ([]*appsv1.ControllerRevision, error) {
	selectorLabels := labels.NewBuilder().WithWorkerSelectorLabels(nodeset).Build()
	selector := k8slabels.SelectorFromSet(k8slabels.Set(selectorLabels))
	return r.historyControl.ListControllerRevisions(nodeset, selector)
}

// getNodeSetPods returns nodeset pods owned by the given nodeset.
// This also reconciles ControllerRef by adopting/orphaning.
// Note that returned histories are pointers to objects in the cache.
// If you want to modify one, you need to deep-copy it first.
func (r *NodeSetReconciler) getNodeSetPods(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
) ([]*corev1.Pod, error) {
	selectorLabels := labels.NewBuilder().WithWorkerSelectorLabels(nodeset).Build()
	selector := k8slabels.SelectorFromSet(k8slabels.Set(selectorLabels))

	// List all pods to include those that do not match the selector anymore but
	// have a ControllerRef pointing to this controller.
	opts := &client.ListOptions{
		Namespace:     nodeset.GetNamespace(),
		LabelSelector: k8slabels.Everything(),
	}
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, opts); err != nil {
		return nil, err
	}
	pods := structutils.ReferenceList(podList.Items)

	filter := func(pod *corev1.Pod) bool {
		// Only claim if it matches our NodeSet name schema. Otherwise release/ignore.
		return nodesetutils.IsPodFromNodeSet(nodeset, pod)
	}

	podControl := podcontrol.NewPodControl(r.Client, r.eventRecorder)

	// Use ControllerRefManager to adopt/orphan as needed.
	cm := kubecontroller.NewPodControllerRefManager(podControl, nodeset, selector, slinkyv1beta1.NodeSetGVK, r.canAdoptFunc(nodeset))
	return cm.ClaimPods(ctx, pods, filter)
}

// If any adoptions are attempted, we should first recheck for deletion with
// an uncached quorum read sometime after listing Pods/ControllerRevisions.
func (r *NodeSetReconciler) canAdoptFunc(nodeset *slinkyv1beta1.NodeSet) func(ctx context.Context) error {
	return kubecontroller.RecheckDeletionTimestamp(func(ctx context.Context) (metav1.Object, error) {
		namespacedName := types.NamespacedName{
			Namespace: nodeset.GetNamespace(),
			Name:      nodeset.GetName(),
		}
		fresh := &slinkyv1beta1.NodeSet{}
		if err := r.Get(ctx, namespacedName, fresh); err != nil {
			return nil, err
		}
		if fresh.UID != nodeset.UID {
			return nil, fmt.Errorf("original NodeSet(%s) is gone: got UID(%v), wanted UID(%v)",
				klog.KObj(nodeset), fresh.UID, nodeset.UID)
		}
		return fresh, nil
	})
}

type SyncStep struct {
	Name string
	Sync func(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod, hash string) error
}

// sync is the main reconciliation logic.
func (r *NodeSetReconciler) sync(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) error {
	syncSteps := []SyncStep{
		{
			Name: "RefreshNodeCache",
			Sync: func(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, _ []*corev1.Pod, _ string) error {
				return r.slurmControl.RefreshNodeCache(ctx, nodeset)
			},
		},
		{
			Name: "ClusterWorkerService",
			Sync: func(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, _ []*corev1.Pod, _ string) error {
				return r.syncClusterWorkerService(ctx, nodeset)
			},
		},
		{
			Name: "ClusterWorkerPDB",
			Sync: func(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, _ []*corev1.Pod, _ string) error {
				return r.syncClusterWorkerPDB(ctx, nodeset)
			},
		},
		{
			Name: "SSHConfig",
			Sync: func(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, _ []*corev1.Pod, _ string) error {
				return r.syncSshConfig(ctx, nodeset)
			},
		},
		{
			Name: "SlurmNodes",
			Sync: func(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod, _ string) error {
				return r.syncSlurmNodes(ctx, nodeset, pods)
			},
		},
		{
			Name: "SlurmDeadline",
			Sync: func(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod, _ string) error {
				return r.syncSlurmDeadline(ctx, nodeset, pods)
			},
		},
		{
			Name: "SlurmTopology",
			Sync: func(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod, _ string) error {
				return r.syncSlurmTopology(ctx, nodeset, pods)
			},
		},
		{
			Name: "Cordon",
			Sync: func(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod, _ string) error {
				return r.syncCordon(ctx, nodeset, pods)
			},
		},
		{
			Name: "NodeTaint",
			Sync: func(ctx context.Context, _ *slinkyv1beta1.NodeSet, _ []*corev1.Pod, _ string) error {
				return r.syncNodeTaint(ctx)
			},
		},
		{
			Name: "NodeSetPods",
			Sync: func(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod, hash string) error {
				return r.syncNodeSetPods(ctx, nodeset, pods, hash)
			},
		},
	}

	for _, s := range syncSteps {
		if err := s.Sync(ctx, nodeset, pods, hash); err != nil {
			msg := fmt.Sprintf("Failed %q step: %v", s.Name, err)
			r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeWarning, SyncFailedReason, "Sync", msg)
			return err
		}
	}

	return nil
}

// syncClusterWorkerService manages the cluster worker hostname service for the Slurm cluster.
func (r *NodeSetReconciler) syncClusterWorkerService(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) error {
	service, err := r.builder.BuildClusterWorkerService(nodeset)
	if err != nil {
		return fmt.Errorf("failed to build cluster worker service: %w", err)
	}

	serviceKey := client.ObjectKeyFromObject(service)
	if err := r.Get(ctx, serviceKey, service); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	clusterName := nodeset.Spec.ControllerRef.Name
	if err := nodesetutils.SetOwnerReferences(r.Client, ctx, service, clusterName); err != nil {
		return err
	}

	if err := objectutils.SyncObject(r.Client, ctx, nil, nil, service, true); err != nil {
		return fmt.Errorf("failed to sync service (%s): %w", klog.KObj(service), err)
	}

	return nil
}

// syncCordon handles propagating cordon/uncordon activity into the NodeSet pods.
//
// When the Kubernetes node is cordoned, the NodeSet pods on that node should have their Slurm node drained.
// Conversely, when the Kubernetes node is uncordoned, the NodeSet pods on that node should have their Slurm node be undrained.
// Otherwise the pods' pod-cordon label intent is propagated -- have the Slurm node drained or undrained.
func (r *NodeSetReconciler) syncCordon(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	syncCordonFn := func(i int) error {
		pod := pods[i]

		node := &corev1.Node{}
		nodeKey := types.NamespacedName{Name: pod.Spec.NodeName}
		if err := r.Get(ctx, nodeKey, node); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		nodeIsCordoned := node.Spec.Unschedulable
		podIsCordoned := podutils.IsPodCordon(pod)
		slurmNodeIsUnresponsive, err := r.slurmControl.IsNodeDownForUnresponsive(ctx, nodeset, pod)
		if err != nil {
			return err
		}
		ourReason, err := r.slurmControl.IsNodeReasonOurs(ctx, nodeset, pod)
		if err != nil {
			return err
		}

		switch {
		// If Slurm node was externally set into a state, preserve it
		case !ourReason, slurmNodeIsUnresponsive:
			return nil

		// If Kubernetes node is cordoned but pod isn't, cordon the pod
		case nodeIsCordoned:
			logger.Info("Kubernetes node cordoned externally, cordoning pod",
				"pod", klog.KObj(pod), "node", node.Name)
			reason := fmt.Sprintf("Node (%s) was cordoned, Pod (%s) must be cordoned",
				pod.Spec.NodeName, klog.KObj(pod))

			// If the node being cordoned has AnnotationNodeCordonReason set, override the default reason
			node := &corev1.Node{}
			name := pod.Spec.NodeName
			key := types.NamespacedName{
				Name: name,
			}
			if err := r.Get(ctx, key, node); err != nil {
				return fmt.Errorf("failed to get node: %w", err)
			}
			if value, ok := node.Annotations[slinkyv1beta1.AnnotationNodeCordonReason]; ok {
				logger.V(1).Info("Slurm node drain reason overridden by Kubernetes node annotation",
					"reason", value)
				reason = value
			}

			r.eventRecorder.Eventf(nodeset, pod, corev1.EventTypeNormal, NodeCordonReason, "",
				"Cordoning Pod %s: Kubernetes node %s was cordoned", klog.KObj(pod), name)

			if err := r.makePodCordonAndDrain(ctx, nodeset, pod, reason); err != nil {
				return err
			}

		// If pod is cordoned, drain the Slurm node
		case podIsCordoned:
			reason := fmt.Sprintf("Pod (%s) was cordoned", klog.KObj(pod))
			if err := r.slurmControl.MakeNodeDrain(ctx, nodeset, pod, reason); err != nil {
				return err
			}

		// If pod is uncordoned, undrain the Slurm node
		case !podIsCordoned:
			reason := fmt.Sprintf("Pod (%s) was uncordoned", klog.KObj(pod))
			if err := r.slurmControl.MakeNodeUndrain(ctx, nodeset, pod, reason); err != nil {
				return err
			}
		}

		return nil
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, syncCordonFn); err != nil {
		return err
	}

	return nil
}

// syncNodeTaint ensures that a NoExecute taint is applied to all nodes running NodeSets
func (r *NodeSetReconciler) syncNodeTaint(
	ctx context.Context,
) error {
	// Build a list of Kube nodes
	kubeNodeList := &corev1.NodeList{}
	if err := r.List(ctx, kubeNodeList); err != nil {
		return err
	}

	// Build a list NodeSets and NodeSet UIDs
	nodesetList := &slinkyv1beta1.NodeSetList{}
	if err := r.List(ctx, nodesetList); err != nil {
		return err
	}
	nodesetUIDs := sets.New[types.UID]()
	for _, nodeset := range nodesetList.Items {
		if nodeset.Spec.TaintKubeNodes {
			nodesetUIDs.Insert(nodeset.UID)
		}
	}

	// Build a list of Pods
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList); err != nil {
		return err
	}

	// Build a set of Kube Nodes that have NodeSet pods that are not terminating
	// Use GetControllerOf on the pod
	nodeSetWithPod := sets.New[string]()
	for _, pod := range podList.Items {
		owner := metav1.GetControllerOf(&pod)
		if owner == nil || !nodesetUIDs.Has(owner.UID) || !podutils.IsRunning(&pod) || podutils.IsTerminating(&pod) {
			continue
		}
		nodeSetWithPod.Insert(pod.Spec.NodeName)
	}

	syncTaintFn := func(i int) error {
		node := kubeNodeList.Items[i]
		var toUpdate *corev1.Node
		var updated bool
		var err error

		// Taint the node if it has a NodeSet pod that is not terminating
		if nodeSetWithPod.Has(node.Name) {
			toUpdate, updated, err = taints.AddOrUpdateTaint(&node, &slurmtaints.TaintNodeWorker)
			if err != nil {
				return err
			}
		} else {
			// Remove the taint from nodes that don't have NodeSet pods
			toUpdate, updated, err = taints.RemoveTaint(&node, &slurmtaints.TaintNodeWorker)
			if err != nil {
				return err
			}
		}

		if !updated {
			return nil
		}

		patch := client.StrategicMergeFrom(&node)
		if err := r.Patch(ctx, toUpdate, patch); err != nil {
			return err
		}

		return nil
	}
	if _, err := utils.SlowStartBatch(len(kubeNodeList.Items), utils.SlowStartInitialBatchSize, syncTaintFn); err != nil {
		return err
	}

	return nil
}

// syncSlurmNodes handles Slurm node drift where nodes may become unregistered but its pod is running and healthy.
func (r *NodeSetReconciler) syncSlurmNodes(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	registeredSlurmNodes, ok, err := r.slurmControl.GetNodesForPods(ctx, nodeset, pods)
	if err != nil {
		return err
	} else if !ok {
		return nil // skip, results cannot be used
	}
	registeredSlurmNodeSet := set.New(registeredSlurmNodes...)

	syncSlurmNodesFn := func(i int) error {
		pod := pods[i]
		isRegistered := registeredSlurmNodeSet.Has(nodesetutils.GetSlurmNodeName(pod))
		if isRegistered ||
			!podutils.IsRunningAndAvailable(pod, nodeset.Spec.MinReadySeconds) ||
			!podutils.IsHealthy(pod) {
			// Cannot determine if Slurm node should be registered at this time.
			return nil
		}
		logger.Info("Deleting NodeSet pod, Slurm node is not registered but pod is healthy",
			"pod", klog.KObj(pod))
		r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeWarning, SlurmNodeNotRegisteredReason, "",
			"Deleting Pod %s: Slurm node is not registered but pod is healthy", klog.KObj(pod))
		if err := r.Delete(ctx, pod); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, syncSlurmNodesFn); err != nil {
		return err
	}

	return nil
}

// syncSlurmDeadline handles the Slurm Node's workload completion deadline.
func (r *NodeSetReconciler) syncSlurmDeadline(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
) error {
	nodeDeadlines, err := r.slurmControl.GetNodeDeadlines(ctx, nodeset, pods)
	if err != nil {
		return err
	}

	syncSlurmDeadlineFn := func(i int) error {
		pod := pods[i]
		slurmNodeName := nodesetutils.GetSlurmNodeName(pod)
		deadline := nodeDeadlines.Peek(slurmNodeName)

		toUpdate := pod.DeepCopy()
		if deadline.IsZero() {
			delete(toUpdate.Annotations, slinkyv1beta1.AnnotationPodDeadline)
		} else {
			toUpdate.Annotations[slinkyv1beta1.AnnotationPodDeadline] = deadline.Format(time.RFC3339)
		}
		if err := r.Patch(ctx, toUpdate, client.StrategicMergeFrom(pod)); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		return nil
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, syncSlurmDeadlineFn); err != nil {
		return err
	}

	return nil
}

// syncSlurmTopology handles the Slurm Node's topology.
func (r *NodeSetReconciler) syncSlurmTopology(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	syncSlurmTopologyFn := func(i int) error {
		pod := pods[i]

		if pod.Spec.NodeName == "" {
			// Skip if Pod has not been allocated to a Node.
			return nil
		}

		node := &corev1.Node{}
		nodeKey := types.NamespacedName{Name: pod.Spec.NodeName}
		if err := r.Get(ctx, nodeKey, node); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		topologyLine := node.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec]

		toUpdate := pod.DeepCopy()
		toUpdate.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec] = topologyLine
		if err := r.Patch(ctx, toUpdate, client.StrategicMergeFrom(pod)); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			logger.Error(err, "failed to patch pod annotations", "pod", klog.KObj(pod))
			return err
		}

		if err := r.slurmControl.UpdateNodeTopology(ctx, nodeset, pod, topologyLine); err != nil {
			// Best effort, no guarantee the topology is valid from the admin.
			logger.Error(err, "failed to update Slurm node topology", "pod", klog.KObj(pod))
		}

		return nil
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, syncSlurmTopologyFn); err != nil {
		return err
	}

	return nil
}

// EnqueueNodeSetAfter schedules a reconcile of the NodeSet after the given delay.
// It uses the shared durationStore so that the next Reconcile result will have RequeueAfter set.
func (r *NodeSetReconciler) EnqueueNodeSetAfter(nodeset *slinkyv1beta1.NodeSet, after time.Duration) {
	key := objectutils.KeyFunc(nodeset)
	durationStore.Push(key, after)
}

func (r *NodeSetReconciler) getDesiredNodeCountForDaemonSet(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) (int32, error) {
	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		return 0, err
	}
	var count int32
	for i := range nodeList.Items {
		shouldRun, _ := r.NodeShouldRunDaemonPod(ctx, &nodeList.Items[i], nodeset)
		if shouldRun {
			count++
		}
	}
	return count, nil
}

func (r *NodeSetReconciler) getNodesToDaemonPods(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod, includeDeletedTerminal bool) map[string][]*corev1.Pod {
	// Group Pods by Node name.
	nodeToDaemonPods := make(map[string][]*corev1.Pod)
	logger := klog.FromContext(ctx)
	for _, pod := range pods {
		if !includeDeletedTerminal && podutil.IsPodTerminal(pod) && pod.DeletionTimestamp != nil {
			// This Pod has a finalizer or is already scheduled for deletion from the
			// store by the kubelet or the Pod GC. The DS controller doesn't have
			// anything else to do with it.
			continue
		}
		nodeName, err := daemonutils.GetTargetNodeName(pod)
		if err != nil {
			logger.V(4).Info("Failed to get target node name of Pod in NodeSet",
				"pod", klog.KObj(pod), "daemonset", klog.KObj(nodeset))
			continue
		}

		nodeToDaemonPods[nodeName] = append(nodeToDaemonPods[nodeName], pod)
	}

	return nodeToDaemonPods
}

func predicates(logger klog.Logger, pod *corev1.Pod, node *corev1.Node, taints []corev1.Taint) (fitsNodeName, fitsNodeAffinity, fitsTaints bool) {
	fitsNodeName = len(pod.Spec.NodeName) == 0 || pod.Spec.NodeName == node.Name
	// Ignore parsing errors for backwards compatibility.
	fitsNodeAffinity, _ = nodeaffinity.GetRequiredNodeAffinity(pod).Match(node)
	_, hasUntoleratedTaint := v1helper.FindMatchingUntoleratedTaint(logger, taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
		return t.Effect == corev1.TaintEffectNoExecute || t.Effect == corev1.TaintEffectNoSchedule
	}, utilfeature.DefaultFeatureGate.Enabled(features.TaintTolerationComparisonOperators))
	fitsTaints = !hasUntoleratedTaint
	return
}

// NodeShouldRunDaemonPod checks a set of preconditions against a (node,daemonset) and returns a
// summary. Returned booleans are:
//   - shouldRun:
//     Returns true when a daemonset should run on the node if a daemonset pod is not already
//     running on that node.
//   - shouldContinueRunning:
//     Returns true when a daemonset should continue running on a node if a daemonset pod is already
//     running on that node.
func (r *NodeSetReconciler) NodeShouldRunDaemonPod(ctx context.Context, node *corev1.Node, nodeset *slinkyv1beta1.NodeSet) (bool, bool) {
	logger := log.FromContext(ctx)
	pod, err := r.newNodeSetPodDaemon(r.Client, ctx, nodeset, node.Name, "")
	if err != nil {
		return false, false
	}

	taints := node.Spec.Taints
	fitsNodeName, fitsNodeAffinity, fitsTaints := predicates(logger, pod, node, taints)
	if !fitsNodeName || !fitsNodeAffinity {
		return false, false
	}

	if !fitsTaints {
		// Scheduled daemon pods should continue running if they tolerate NoExecute taint.
		_, hasUntoleratedTaint := v1helper.FindMatchingUntoleratedTaint(logger, taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
			return t.Effect == corev1.TaintEffectNoExecute
		}, utilfeature.DefaultFeatureGate.Enabled(features.TaintTolerationComparisonOperators))
		return false, !hasUntoleratedTaint
	}

	return true, true
}

func failedPodsBackoffKey(nodeset *slinkyv1beta1.NodeSet, nodeName string) string {
	return fmt.Sprintf("%s/%d/%s", nodeset.UID, nodeset.Status.ObservedGeneration, nodeName)
}

func (r *NodeSetReconciler) podsShouldBeOnNode(
	ctx context.Context,
	node *corev1.Node,
	nodeToDaemonPods map[string][]*corev1.Pod,
	nodeset *slinkyv1beta1.NodeSet,
) (nodesNeedingDaemonPods []string, podsToDelete []*corev1.Pod) {

	logger := log.FromContext(ctx)
	shouldRun, shouldContinueRunning := r.NodeShouldRunDaemonPod(ctx, node, nodeset)
	daemonPods, exists := nodeToDaemonPods[node.Name]

	switch {
	case shouldRun && !exists:
		// If daemon pod is supposed to be running on node, but isn't, create daemon pod.
		nodesNeedingDaemonPods = append(nodesNeedingDaemonPods, node.Name)
	case shouldContinueRunning:
		// If a daemon pod failed, delete it
		// If there's non-daemon pods left on this node, we will create it in the next sync loop
		var daemonPodsRunning []*corev1.Pod
		for _, pod := range daemonPods {
			switch {
			case pod.DeletionTimestamp != nil:
				continue

			case pod.Status.Phase == corev1.PodFailed:
				// This is a critical place where the controller often fights with kubelet that rejects pods.
				// We need to avoid hot looping and backoff.
				backoffKey := failedPodsBackoffKey(nodeset, node.Name)

				now := failedPodsBackoff.Clock.Now()
				inBackoff := failedPodsBackoff.IsInBackOffSinceUpdate(backoffKey, now)
				if inBackoff {
					delay := failedPodsBackoff.Get(backoffKey)
					logger.V(4).Info("Deleting failed pod on node has been limited by backoff",
						"pod", klog.KObj(pod), "node", klog.KObj(node), "currentDelay", delay)
					r.EnqueueNodeSetAfter(nodeset, delay)
					continue
				}

				failedPodsBackoff.Next(backoffKey, now)

				msg := fmt.Sprintf("Found failed daemon pod %s/%s on node %s, will try to kill it", pod.Namespace, pod.Name, node.Name)
				logger.V(2).Info("Found failed daemon pod on node, will try to kill it", "pod", klog.KObj(pod), "node", klog.KObj(node))
				// Emit an event so that it's discoverable to users.
				r.eventRecorder.Eventf(nodeset, pod, corev1.EventTypeWarning, FailedDaemonPodReason, "Info", msg)
				podsToDelete = append(podsToDelete, pod)

			case pod.Status.Phase == corev1.PodSucceeded:
				msg := fmt.Sprintf("Found succeeded daemon pod %s/%s on node %s, will try to delete it", pod.Namespace, pod.Name, node.Name)
				logger.V(2).Info("Found succeeded daemon pod on node, will try to delete it", "pod", klog.KObj(pod), "node", klog.KObj(node))
				// Emit an event so that it's discoverable to users.
				r.eventRecorder.Eventf(nodeset, pod, corev1.EventTypeNormal, SucceededDaemonPodReason, "Info", msg)
				podsToDelete = append(podsToDelete, pod)

			default:
				daemonPodsRunning = append(daemonPodsRunning, pod)
			}
		}

		// NodeSet allows at most one pod per node. If there is more than one running pod, delete all but the oldest.
		if len(daemonPodsRunning) <= 1 {
			break
		}
		sort.Sort(nodesetutils.ActivePods(daemonPodsRunning))
		for i := 1; i < len(daemonPodsRunning); i++ {
			podsToDelete = append(podsToDelete, daemonPodsRunning[i])
		}

	case !shouldContinueRunning && exists:
		// If daemon pod isn't supposed to run on node, but it is, delete all daemon pods on node.
		for _, pod := range daemonPods {
			if pod.DeletionTimestamp != nil {
				continue
			}
			podsToDelete = append(podsToDelete, pod)
		}
	}

	return nodesNeedingDaemonPods, podsToDelete
}

// syncNodeSetPods will reconcile NodeSet pod replica counts.
// Pods will be:
//   - Scaled out when: `replicaCount < replicasWant“
//   - Scaled in when: `replicaCount > replicasWant“
//   - Processed when: `replicaCount == replicasWant“
func (r *NodeSetReconciler) syncNodeSetPods(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) error {
	logger := log.FromContext(ctx)

	// Delete pods that were created for a different ScalingMode
	// (e.g. after switching nodeset from statefulset to daemonset or vice versa).
	var podsOldScaling, podsNewScaling []*corev1.Pod
	for _, pod := range pods {
		podMode := slinkyv1beta1.ScalingModeType(pod.Labels[slinkyv1beta1.LabelNodeSetScalingMode])
		if podMode != nodeset.Spec.ScalingMode {
			podsOldScaling = append(podsOldScaling, pod)
		} else {
			podsNewScaling = append(podsNewScaling, pod)
		}
	}

	if nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeDaemonset {
		logger.V(2).Info("Processing NodeSet pods in DaemonSet mode")
		nodeList := &corev1.NodeList{}
		if err := r.List(ctx, nodeList); err != nil {
			return err
		}
		nodeToDaemonPods := r.getNodesToDaemonPods(ctx, nodeset, podsNewScaling, false)
		var nodesNeedingDaemonPods []string
		var podsToDelete []*corev1.Pod
		for _, node := range nodeList.Items {
			nodesNeedingDaemonPodsOnNode, podsToDeleteOnNode := r.podsShouldBeOnNode(
				ctx, &node, nodeToDaemonPods, nodeset)

			nodesNeedingDaemonPods = append(nodesNeedingDaemonPods, nodesNeedingDaemonPodsOnNode...)
			podsToDelete = append(podsToDelete, podsToDeleteOnNode...)
		}
		podsToCreate := make([]*corev1.Pod, len(nodesNeedingDaemonPods))
		for i := range len(nodesNeedingDaemonPods) {
			pod, err := r.newNodeSetPodDaemon(r.Client, ctx, nodeset, nodesNeedingDaemonPods[i], hash)
			if err != nil {
				return err
			}
			podsToCreate[i] = pod
		}
		if len(podsToDelete) > 0 || len(podsToCreate) > 0 {
			if len(podsToCreate) > 0 {
				r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeNormal, ScalingUpReason, "ScaleUp",
					"Creating %d daemon Pod(s)", len(podsToCreate))
			}
			if len(podsToDelete) > 0 {
				r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeNormal, ScalingDownReason, "ScaleDown",
					"Deleting %d daemon Pod(s)", len(podsToDelete))
			}
			return r.doPodScale(ctx, nodeset, podsNewScaling, podsToDelete, podsToCreate)
		}
	} else {
		logger.V(2).Info("Processing NodeSet pods in StatefulSet mode")

		// Handle replica scaling by comparing the known pods to the target number of replicas.
		// Create or delete pods as needed to reach the target number.
		replicaCount := int(ptr.Deref(nodeset.Spec.Replicas, defaults.DefaultNodeSetReplicas))
		diff := len(podsNewScaling) - replicaCount
		if diff < 0 {
			diff = -diff

			podsToCreate := make([]*corev1.Pod, diff)
			usedOrdinals := set.New[int]()
			for _, pod := range pods {
				usedOrdinals.Insert(nodesetutils.GetOrdinal(pod))
			}
			ordinal := 0
			for i := range diff {
				for usedOrdinals.Has(ordinal) {
					ordinal++
				}
				pod, err := r.newNodeSetPodOrdinal(r.Client, ctx, nodeset, ordinal, hash)
				if err != nil {
					return err
				}
				usedOrdinals.Insert(ordinal)
				podsToCreate[i] = pod
			}
			logger.V(2).Info("Too few NodeSet pods", "need", replicaCount, "creating", diff)
			r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeNormal, ScalingUpReason, "ScaleUp",
				"Creating %d Pod(s) to stabilize at %d replicas", diff, replicaCount)
			return r.doPodScale(ctx, nodeset, podsNewScaling, nil, podsToCreate)
		}
		if diff > 0 {
			logger.V(2).Info("Too many NodeSet pods", "need", replicaCount, "deleting", diff)
			r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeNormal, ScalingDownReason, "ScaleDown",
				"Deleting %d Pod(s) to stabilize at %d replicas", diff, replicaCount)
			podsToDelete, podsToKeep := nodesetutils.SplitActivePods(podsNewScaling, diff)
			return r.doPodScale(ctx, nodeset, podsToKeep, podsToDelete, nil)
		}
	}

	logger.V(2).Info("Processing NodeSet pods", "number of pods to process", len(podsNewScaling), "number of pods to delete", len(podsOldScaling))
	return r.doPodProcessing(ctx, nodeset, podsNewScaling, podsOldScaling, hash)
}

// doPodScale manages NodeSet pod creation and deletion
// podsToKeep - should be uncordoned and undrained.
// podsToDelete - should be cordoned and drained, then deleted.
// podsToCreate - should be newly created.
func (r *NodeSetReconciler) doPodScale(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	podsToKeep, podsToDelete, podsToCreate []*corev1.Pod,
) error {
	logger := log.FromContext(ctx)
	key := objectutils.KeyFunc(nodeset)
	errs := []error{}

	numDelete := mathutils.Clamp(len(podsToDelete), 0, burstReplicas)
	numCreate := mathutils.Clamp(len(podsToCreate), 0, burstReplicas)

	// Snapshot the UIDs (namespace/name) of the pods we're expecting to see
	// deleted, so we know to record their expectations exactly once either
	// when we see it as an update of the deletion timestamp, or as a delete.
	// Note that if the labels on a pod/nodeset change in a way that the pod gets
	// orphaned, the nodeset will only wake up after the expectations have
	// expired even if other pods are deleted.
	if err := r.expectations.ExpectDeletions(logger, key, getPodKeys(podsToDelete[:numDelete])); err != nil {
		return err
	}

	// TODO: Track UIDs of creates just like deletes. The problem currently
	// is we'd need to wait on the result of a create to record the pod's
	// UID, which would require locking *across* the create, which will turn
	// into a performance bottleneck. We should generate a UID for the pod
	// beforehand and store it via ExpectCreations.
	r.expectations.RaiseExpectations(logger, key, len(podsToCreate[:numCreate]), 0)

	uncordonFn := func(i int) error {
		pod := podsToKeep[i]
		return r.syncPodUncordon(ctx, nodeset, pod)
	}
	if _, err := utils.SlowStartBatch(len(podsToKeep), utils.SlowStartInitialBatchSize, uncordonFn); err != nil {
		return err
	}

	// Batch the pod creates. Batch sizes start at SlowStartInitialBatchSize
	// and double with each successful iteration in a kind of "slow start".
	// This handles attempts to start large numbers of pods that would
	// likely all fail with the same error. For example a project with a
	// low quota that attempts to create a large number of pods will be
	// prevented from spamming the API service with the pod create requests
	// after one of its pods fails. Conveniently, this also prevents the
	// event spam that those failures would generate.
	createPodFn := func(index int) error {
		pod := podsToCreate[index]
		if err := r.podControl.CreateNodeSetPod(ctx, nodeset, pod); err != nil {
			if apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
				// if the namespace is being terminated, we don't have to do
				// anything because any creation will fail
				return nil
			}
			return err
		}
		return nil
	}
	successfulCreations, err := utils.SlowStartBatch(numCreate, utils.SlowStartInitialBatchSize, createPodFn)
	if err != nil {
		errs = append(errs, err)
	}

	// Any skipped pods that we never attempted to start shouldn't be expected.
	// The skipped pods will be retried later. The next controller resync will
	// retry the slow start process.
	if skippedPods := numCreate - successfulCreations; skippedPods > 0 {
		logger.V(2).Info("Slow-start failure. Skipping creation of pods, decrementing expectations",
			"podsSkipped", skippedPods, "kind", slinkyv1beta1.NodeSetGVK)
		for range skippedPods {
			// Decrement the expected number of creates because the informer won't observe this pod
			r.expectations.CreationObserved(logger, key)
		}
	}

	fixPodPVCsFn := func(i int) error {
		pod := podsToDelete[i]
		if matchPolicy, err := r.podControl.PodPVCsMatchRetentionPolicy(ctx, nodeset, pod); err != nil {
			return err
		} else if !matchPolicy {
			if err := r.podControl.UpdatePodPVCsForRetentionPolicy(ctx, nodeset, pod); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := utils.SlowStartBatch(len(podsToDelete), utils.SlowStartInitialBatchSize, fixPodPVCsFn); err != nil {
		errs = append(errs, err)
	}

	deletePodFn := func(index int) error {
		pod := podsToDelete[index]
		podKey := kubecontroller.PodKey(pod)
		if err := r.processCondemned(ctx, nodeset, podsToDelete, index); err != nil {
			// Decrement the expected number of deletes because the informer won't observe this deletion
			r.expectations.DeletionObserved(logger, key, podKey)
			if !apierrors.IsNotFound(err) {
				logger.V(2).Info("Failed to delete pod, decremented expectations",
					"pod", podKey, "kind", slinkyv1beta1.NodeSetGVK)
				return err
			}
		}
		return nil
	}
	if _, err := utils.SlowStartBatch(numDelete, utils.SlowStartInitialBatchSize, deletePodFn); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func (r *NodeSetReconciler) newNodeSetPodDaemon(
	client client.Client,
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	nodeName string,
	revisionHash string,
) (*corev1.Pod, error) {
	controller := &slinkyv1beta1.Controller{}
	key := nodeset.Spec.ControllerRef.NamespacedName()
	if err := r.Get(ctx, key, controller); err != nil {
		r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeWarning, ControllerRefFailedReason, "Info",
			"Failed to get Controller (%s): %v", key, err)
		return nil, err
	}
	if nodeName == "" {
		return nil, fmt.Errorf("nodeName must not be empty")
	}

	pod := nodesetutils.NewNodeSetDaemonSetPod(client, nodeset, controller, nodeName, revisionHash)
	return pod, nil
}

func (r *NodeSetReconciler) newNodeSetPodOrdinal(
	client client.Client,
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	ordinal int,
	revisionHash string,
) (*corev1.Pod, error) {
	controller := &slinkyv1beta1.Controller{}
	key := nodeset.Spec.ControllerRef.NamespacedName()
	if err := r.Get(ctx, key, controller); err != nil {
		r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeWarning, ControllerRefFailedReason, "Info",
			"Failed to get Controller (%s): %v", key, err)
		return nil, err
	}

	pod := nodesetutils.NewNodeSetStatefulSetPod(client, nodeset, controller, ordinal, revisionHash)

	return pod, nil
}

func getPodKeys(pods []*corev1.Pod) []string {
	podKeys := make([]string, 0, len(pods))
	for _, pod := range pods {
		podKeys = append(podKeys, kubecontroller.PodKey(pod))
	}
	return podKeys
}

// processCondemned will gracefully terminate the condemned NodeSet pod.
// NOTE: intended to be used by utils.SlowStartBatch().
func (r *NodeSetReconciler) processCondemned(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	condemned []*corev1.Pod,
	i int,
) error {
	logger := klog.FromContext(ctx)
	pod := condemned[i]

	podKey := client.ObjectKeyFromObject(pod)
	if err := r.Get(ctx, podKey, pod); err != nil {
		return err
	}

	if podutils.IsTerminating(pod) {
		logger.V(3).Info("NodeSet Pod is terminating, skipping further processing",
			"pod", klog.KObj(pod))
		return nil
	}

	isDrained, err := r.slurmControl.IsNodeDrained(ctx, nodeset, pod)
	if err != nil {
		return err
	}

	if !isDrained {
		logger.V(2).Info("NodeSet Pod is draining, pending termination for scale-in",
			"pod", klog.KObj(pod))
		// Decrement expectations and requeue reconcile because the Slurm node is not drained yet.
		// We must wait until fully drained to terminate the pod.
		nodesetKey := objectutils.KeyFunc(nodeset)
		durationStore.Push(nodesetKey, 30*time.Second)
		r.expectations.DeletionObserved(logger, nodesetKey, kubecontroller.PodKey(pod))
		reason := fmt.Sprintf("Pod (%s) was cordoned pending termination", klog.KObj(pod))
		return r.makePodCordonAndDrain(ctx, nodeset, pod, reason)
	}

	logger.V(2).Info("NodeSet Pod is terminating for scale-in",
		"pod", klog.KObj(pod))
	if err := r.podControl.DeleteNodeSetPod(ctx, nodeset, pod); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// doPodProcessing handles batch processing of NodeSet pods.
func (r *NodeSetReconciler) doPodProcessing(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods, podsToDelete []*corev1.Pod,
	hash string,
) error {
	var errs []error
	logger := log.FromContext(ctx)
	key := objectutils.KeyFunc(nodeset)

	if err := r.expectations.SetExpectations(logger, key, 0, 0); err != nil {
		return err
	}

	// NOTE: we must respect the uncordon and undrain nodes in accordance with updateStrategy
	// to not fight it given the statefulness of how we cordon and terminate nodeset pods.
	_, podsToKeep := r.splitUpdatePods(ctx, nodeset, pods, hash)
	uncordonFn := func(i int) error {
		pod := podsToKeep[i]
		return r.syncPodUncordon(ctx, nodeset, pod)
	}
	if _, err := utils.SlowStartBatch(len(podsToKeep), utils.SlowStartInitialBatchSize, uncordonFn); err != nil {
		errs = append(errs, err)
	}

	deletePodFn := func(index int) error {
		pod := podsToDelete[index]
		podKey := kubecontroller.PodKey(pod)
		if err := r.processCondemned(ctx, nodeset, podsToDelete, index); err != nil {
			// Decrement the expected number of deletes because the informer won't observe this deletion
			r.expectations.DeletionObserved(logger, key, podKey)
			if !apierrors.IsNotFound(err) {
				logger.V(2).Info("Failed to delete pod, decremented expectations",
					"pod", podKey, "kind", slinkyv1beta1.NodeSetGVK)
				return err
			}
		}
		return nil
	}
	if _, err := utils.SlowStartBatch(len(podsToDelete), utils.SlowStartInitialBatchSize, deletePodFn); err != nil {
		errs = append(errs, err)
	}

	processNodeSetPodFn := func(i int) error {
		pod := pods[i]
		return r.processNodeSetPod(ctx, nodeset, pod)
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, processNodeSetPodFn); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

// processNodeSetPod will ensure the NodeSet pod can be scheduled and cleanup errant pods.
// NOTE: intended to be used by utils.SlowStartBatch().
func (r *NodeSetReconciler) processNodeSetPod(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pod *corev1.Pod,
) error {
	// Note that pods with phase Succeeded will also trigger this event. This is
	// because final pod phase of evicted or otherwise forcibly stopped pods
	// (e.g. terminated on node reboot) is determined by the exit code of the
	// container, not by the reason for pod termination. We should restart the
	// pod regardless of the exit code.
	if podutils.IsFailed(pod) || podutils.IsSucceeded(pod) {
		if !podutils.IsTerminating(pod) {
			if err := r.podControl.DeleteNodeSetPod(ctx, nodeset, pod); err != nil {
				return err
			}
		}
		// New pod should be generated on the next sync after the current pod is removed from etcd.
		return nil
	}

	return r.podControl.UpdateNodeSetPod(ctx, nodeset, pod)
}

// makePodCordonAndDrain will cordon the pod and drain the corresponding Slurm node.
func (r *NodeSetReconciler) makePodCordonAndDrain(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pod *corev1.Pod,
	reason string,
) error {
	if err := r.makePodCordon(ctx, pod); err != nil {
		return err
	}

	if err := r.syncSlurmNodeDrain(ctx, nodeset, pod, reason); err != nil {
		return err
	}

	return nil
}

// syncSlurmNodeDrain will drain the corresponding Slurm node.
func (r *NodeSetReconciler) syncSlurmNodeDrain(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pod *corev1.Pod,
	message string,
) error {
	logger := log.FromContext(ctx)

	isDrain, err := r.slurmControl.IsNodeDrain(ctx, nodeset, pod)
	if err != nil {
		return err
	}

	if isDrain {
		logger.V(1).Info("Node is drain, skipping drain request")
		return nil
	}

	reason := fmt.Sprintf("Pod (%s) has been cordoned", klog.KObj(pod))
	if message != "" {
		reason = message
	}

	if err := r.slurmControl.MakeNodeDrain(ctx, nodeset, pod, reason); err != nil {
		return err
	}

	return nil
}

// makePodCordon will cordon the pod.
func (r *NodeSetReconciler) makePodCordon(
	ctx context.Context,
	pod *corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	if podutils.IsPodCordon(pod) {
		return nil
	}

	toUpdate := pod.DeepCopy()
	logger.Info("Cordon Pod, pending deletion", "Pod", klog.KObj(toUpdate))
	if toUpdate.Annotations == nil {
		toUpdate.Annotations = make(map[string]string)
	}
	toUpdate.Annotations[slinkyv1beta1.AnnotationPodCordon] = "true"
	if err := r.Patch(ctx, toUpdate, client.StrategicMergeFrom(pod)); err != nil {
		return err
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
		return err
	}

	return nil
}

// makePodUncordonAndUndrain will uncordon the pod and undrain the corresponding Slurm node.
func (r *NodeSetReconciler) makePodUncordonAndUndrain(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pod *corev1.Pod,
	reason string,
) error {
	if err := r.makePodUncordon(ctx, pod); err != nil {
		return err
	}

	if err := r.syncSlurmNodeUndrain(ctx, nodeset, pod, reason); err != nil {
		return err
	}

	return nil
}

// syncSlurmNodeUndrain will undrain the corresponding Slurm node.
func (r *NodeSetReconciler) syncSlurmNodeUndrain(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pod *corev1.Pod,
	message string,
) error {
	logger := log.FromContext(ctx)

	isDrain, err := r.slurmControl.IsNodeDrain(ctx, nodeset, pod)
	if err != nil {
		return err
	}

	if !isDrain {
		logger.V(1).Info("Node is undrain, skipping undrain request")
		return nil
	}

	reason := fmt.Sprintf("Pod (%s) has been uncordoned", klog.KObj(pod))
	if message != "" {
		reason = message
	}

	if err := r.slurmControl.MakeNodeUndrain(ctx, nodeset, pod, reason); err != nil {
		return err
	}

	return nil
}

// makePodUncordonAndUndrain will uncordon the pod.
func (r *NodeSetReconciler) makePodUncordon(ctx context.Context, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)

	if !podutils.IsPodCordon(pod) {
		return nil
	}

	toUpdate := pod.DeepCopy()
	logger.Info("Uncordon Pod", "Pod", klog.KObj(toUpdate))
	delete(toUpdate.Annotations, slinkyv1beta1.AnnotationPodCordon)
	if err := r.Patch(ctx, toUpdate, client.StrategicMergeFrom(pod)); err != nil {
		return err
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
		return err
	}

	return nil
}

// syncPodUncordon handles uncordoning with Kubernetes and Slurm node state synchronization
func (r *NodeSetReconciler) syncPodUncordon(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)

	// The Kubernetes nodes which the pod is on may have been cordoned
	if r.isNodeCordoned(ctx, pod) {
		logger.V(1).Info("Skipping uncordon for pod on externally cordoned node",
			"pod", klog.KObj(pod), "node", pod.Spec.NodeName)
		return nil // Skip
	}

	// Slurm node may have been externally set in down, drain, fail, etc...
	if ok, err := r.slurmControl.IsNodeReasonOurs(ctx, nodeset, pod); err != nil {
		return err
	} else if !ok {
		logger.V(1).Info("Skipping uncordon for pod which has an externally set reason",
			"pod", klog.KObj(pod))
		return nil // Skip
	}

	return r.makePodUncordonAndUndrain(ctx, nodeset, pod, "")
}

// isNodeCordoned returns true if the pod's node is cordoned
func (r *NodeSetReconciler) isNodeCordoned(ctx context.Context, pod *corev1.Pod) bool {
	node := &corev1.Node{}
	nodeKey := types.NamespacedName{Name: pod.Spec.NodeName}
	if err := r.Get(ctx, nodeKey, node); err != nil {
		return false
	}

	return node.Spec.Unschedulable
}

// syncUpdate will synchronize NodeSet pod version updates based on update type.
func (r *NodeSetReconciler) syncUpdate(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) error {
	switch nodeset.Spec.UpdateStrategy.Type {
	default:
		fallthrough
	case slinkyv1beta1.RollingUpdateNodeSetStrategyType:
		return r.syncRollingUpdate(ctx, nodeset, pods, hash)
	case slinkyv1beta1.OnDeleteNodeSetStrategyType:
		// r.syncNodeSet() will handled it on the next reconcile
		return nil
	}
}

// syncRollingUpdate will synchronize rolling updates for NodeSet pods.
func (r *NodeSetReconciler) syncRollingUpdate(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) error {
	logger := log.FromContext(ctx)

	_, oldPods := findUpdatedPods(pods, hash)

	unhealthyPods, healthyPods := nodesetutils.SplitUnhealthyPods(oldPods)
	if len(unhealthyPods) > 0 {
		logger.Info("Delete unhealthy pods for Rolling Update",
			"unhealthyPods", len(unhealthyPods))
		r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeNormal, RollingUpdateReason, "RollingUpdate",
			"Rolling update: deleting %d unhealthy old pod(s)", len(unhealthyPods))
		if err := r.doPodScale(ctx, nodeset, nil, unhealthyPods, nil); err != nil {
			return err
		}
	}

	podsToDelete, _ := r.splitUpdatePods(ctx, nodeset, healthyPods, hash)
	if len(podsToDelete) > 0 {
		logger.Info("Scale-in pods for Rolling Update",
			"delete", len(podsToDelete))
		r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeNormal, RollingUpdateReason, "RollingUpdate",
			"Rolling update: replacing %d old pod(s) with updated revision", len(podsToDelete))
		if err := r.doPodScale(ctx, nodeset, nil, podsToDelete, nil); err != nil {
			return err
		}
	}

	return nil
}

// splitUpdatePods returns two pod lists based on UpdateStrategy type.
func (r *NodeSetReconciler) splitUpdatePods(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) (podsToDelete, podsToKeep []*corev1.Pod) {
	logger := log.FromContext(ctx)

	switch nodeset.Spec.UpdateStrategy.Type {
	default:
		fallthrough
	case slinkyv1beta1.RollingUpdateNodeSetStrategyType:
		newPods, oldPods := findUpdatedPods(pods, hash)

		var numUnavailable int
		now := metav1.Now()
		for _, pod := range newPods {
			if !podutil.IsPodAvailable(pod, nodeset.Spec.MinReadySeconds, now) {
				numUnavailable++
			}
		}

		total := int(ptr.Deref(nodeset.Spec.Replicas, defaults.DefaultNodeSetReplicas))
		if nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeDaemonset {
			total = len(pods)
		}
		maxUnavailable := mathutils.GetScaledValueFromIntOrPercent(nodeset.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable, total, true, 1)
		remainingUnavailable := mathutils.Clamp((maxUnavailable - numUnavailable), 0, maxUnavailable)
		podsToDelete, remainingOldPods := nodesetutils.SplitActivePods(oldPods, remainingUnavailable)

		remainingPods := make([]*corev1.Pod, len(newPods))
		copy(remainingPods, newPods)
		remainingPods = append(remainingPods, remainingOldPods...)

		logger.V(1).Info("calculated pod lists for update",
			"maxUnavailable", maxUnavailable,
			"updatePods", len(podsToDelete),
			"remainingPods", len(remainingPods))
		return podsToDelete, remainingPods
	case slinkyv1beta1.OnDeleteNodeSetStrategyType:
		return nil, nil
	}
}

// findUpdatedPods looks at non-deleted pods and returns two lists, new and old pods, given the hash.
func findUpdatedPods(pods []*corev1.Pod, hash string) (newPods, oldPods []*corev1.Pod) {
	for _, pod := range pods {
		if podutils.IsTerminating(pod) {
			continue
		}
		if historycontrol.GetRevision(pod.GetLabels()) == hash {
			newPods = append(newPods, pod)
		} else {
			oldPods = append(oldPods, pod)
		}
	}
	return newPods, oldPods
}

// syncClusterWorkerPDB will reconcile the cluster's PodDisruptionBudget
func (r *NodeSetReconciler) syncClusterWorkerPDB(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
) error {

	podDisruptionBudget, err := r.builder.BuildClusterWorkerPodDisruptionBudget(nodeset)
	if err != nil {
		return fmt.Errorf("failed to build cluster worker PDB: %w", err)
	}

	pdbKey := client.ObjectKeyFromObject(podDisruptionBudget)
	if err := r.Get(ctx, pdbKey, podDisruptionBudget); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	clusterName := nodeset.Spec.ControllerRef.Name
	if err := nodesetutils.SetOwnerReferences(r.Client, ctx, podDisruptionBudget, clusterName); err != nil {
		return err
	}

	// Sync the PodDisruptionBudget for each cluster
	if err := objectutils.SyncObject(r.Client, ctx, nil, nil, podDisruptionBudget, true); err != nil {
		return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(podDisruptionBudget), err)
	}

	return nil
}

// syncSshConfig manages SSH config for the NodeSet if SSH is enabled
func (r *NodeSetReconciler) syncSshConfig(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
) error {
	// Only create SSH config keys if SSH is enabled
	if !nodeset.Spec.Ssh.Enabled {
		return nil
	}

	config, err := r.builder.BuildWorkerSshConfig(nodeset)
	if err != nil {
		return fmt.Errorf("failed to build SSH config: %w", err)
	}

	if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, nodeset, config, true); err != nil {
		return fmt.Errorf("failed to sync SSH config (%s): %w", klog.KObj(config), err)
	}

	return nil
}
