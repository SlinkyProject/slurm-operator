// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/slurmcontrol"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/SlinkyProject/slurm-operator/internal/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/historycontrol"
	"github.com/SlinkyProject/slurm-operator/internal/utils/mathutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podutils"
	slurmconditions "github.com/SlinkyProject/slurm-operator/pkg/conditions"
)

// syncStatus handles synchronizing Slurm Nodes and NodeSet Status.
func (r *NodeSetReconciler) syncStatus(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
	currentRevision, updateRevision *appsv1.ControllerRevision,
	collisionCount int32,
	hash string,
	errors ...error,
) error {
	if err := r.slurmControl.RefreshNodeCache(ctx, nodeset); err != nil {
		errors = append(errors, err)
	}

	if err := r.syncSlurmStatus(ctx, nodeset, pods); err != nil {
		errors = append(errors, err)
	}

	if err := r.syncNodeSetStatus(ctx, nodeset, pods, currentRevision, updateRevision, collisionCount, hash); err != nil {
		errors = append(errors, err)
	}

	if err := r.syncNodeSetPodStatus(ctx, nodeset, pods); err != nil {
		return err
	}

	return utilerrors.NewAggregate(errors)
}

// syncSlurmStatus handles synchronizing Slurm Node Status given the pods.
func (r *NodeSetReconciler) syncSlurmStatus(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
) error {
	syncSlurmStatusFn := func(i int) error {
		pod := pods[i]
		if !podutils.IsHealthy(pod) {
			return nil
		}
		return r.slurmControl.UpdateNodeWithPodInfo(ctx, nodeset, pod)
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, syncSlurmStatusFn); err != nil {
		return err
	}

	return nil
}

// syncSlurmStatus handles synchronizing NodeSet Status.
func (r *NodeSetReconciler) syncNodeSetStatus(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
	currentRevision, updateRevision *appsv1.ControllerRevision,
	collisionCount int32,
	hash string,
) error {
	logger := log.FromContext(ctx)

	selectorLabels := labels.NewBuilder().WithWorkerSelectorLabels(nodeset).Build()
	selector := k8slabels.SelectorFromSet(k8slabels.Set(selectorLabels))

	replicaStatus, err := r.calculateReplicaStatus(ctx, nodeset, pods, currentRevision, updateRevision)
	if err != nil {
		return err
	}
	slurmNodeStatus, err := r.slurmControl.CalculateNodeStatus(ctx, nodeset, pods)
	if err != nil {
		return err
	}
	ordinalToNode, err := r.calculateOrdinalToNode(ctx, nodeset, pods)
	if err != nil {
		return err
	}

	newStatus := slinkyv1beta1.NodeSetStatus{
		Replicas:            replicaStatus.Replicas,
		UpdatedReplicas:     replicaStatus.Updated,
		ReadyReplicas:       replicaStatus.Ready,
		AvailableReplicas:   replicaStatus.Available,
		UnavailableReplicas: replicaStatus.Unavailable,
		Desired:             replicaStatus.Desired,
		SlurmIdle:           slurmNodeStatus.Idle,
		SlurmAllocated:      slurmNodeStatus.Allocated + slurmNodeStatus.Mixed,
		SlurmDown:           slurmNodeStatus.Down,
		SlurmDrain:          slurmNodeStatus.Drain,
		ObservedGeneration:  nodeset.Generation,
		NodeSetHash:         hash,
		CollisionCount:      &collisionCount,
		OrdinalToNode:       ordinalToNode,
		ConfigHashes:        configHashesFromTemplate(nodeset),
		Selector:            selector.String(),
		Conditions:          []metav1.Condition{},
	}
	newStatus.Conditions = append(newStatus.Conditions, nodeset.Status.Conditions...)

	if err := r.applyReservationCondition(ctx, nodeset, &newStatus.Conditions); err != nil {
		return err
	}

	if apiequality.Semantic.DeepEqual(nodeset.Status, newStatus) {
		logger.V(2).Info("NodeSet Status has not changed, skipping status update", "status", nodeset.Status)
		return nil
	}

	if err := r.updateNodeSetStatus(ctx, nodeset, &newStatus); err != nil {
		return err
	}

	key := objectutils.KeyFunc(nodeset)
	if nodeset.Spec.MinReadySeconds >= 0 && (newStatus.ReadyReplicas != newStatus.AvailableReplicas) {
		// Resync the NodeSet after MinReadySeconds as a last line of defense to guard against clock-skew.
		durationStore.Push(key, (time.Duration(nodeset.Spec.MinReadySeconds)*time.Second)+time.Second)
	} else if slurmNodeStatus.Total != newStatus.Replicas {
		// Resync the NodeSet until the Slurm counts are correct.
		durationStore.Push(key, 10*time.Second)
	}

	return nil
}

type replicaStatus struct {
	Replicas    int32
	Ready       int32
	Available   int32
	Unavailable int32
	Current     int32
	Updated     int32
	Desired     int32
}

// calculateReplicaStatus will calculate the status of the given pods.
func (r *NodeSetReconciler) calculateReplicaStatus(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
	currentRevision, updateRevision *appsv1.ControllerRevision,
) (replicaStatus, error) {
	status := replicaStatus{}

	now := metav1.Now()
	for _, pod := range pods {
		// Count the Replicas
		if podutils.IsCreated(pod) {
			status.Replicas++
		}
		// Count the Ready and Available replicas
		if podutils.IsRunningAndReady(pod) {
			status.Ready++
			if podutil.IsPodAvailable(pod, nodeset.Spec.MinReadySeconds, now) {
				status.Available++
			}
		}
		// Count the Current and Updated replicas
		if podutils.IsCreated(pod) && !podutils.IsTerminating(pod) {
			podHash := historycontrol.GetRevision(pod.GetLabels())
			curRevHash := historycontrol.GetRevision(currentRevision.GetLabels())
			newRevHash := historycontrol.GetRevision(updateRevision.GetLabels())
			if podHash == curRevHash {
				status.Current++
			}
			if podHash == newRevHash {
				status.Updated++
			}
		}
	}
	if nodeset != nil && nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeStatefulset {
		status.Unavailable = mathutils.Clamp(status.Replicas-status.Available, 0, status.Replicas)
		status.Desired = ptr.Deref(nodeset.Spec.Replicas, defaults.DefaultNodeSetReplicas)
	} else if nodeset != nil && nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeDaemonset {
		desiredNodes, err := r.getDesiredNodeCountForDaemonSet(ctx, nodeset)
		if err != nil {
			return status, fmt.Errorf("failed to get desired node count for daemon set: %w", err)
		}
		status.Unavailable = mathutils.Clamp(desiredNodes-status.Available, 0, desiredNodes)
		status.Desired = int32(desiredNodes)
	}
	return status, nil
}

// calculateReservationCondition returns a condition that indicates the status of the reservation
// associated with the NodeSet.
//
// NodeSetConditionReservationCreated conveys three pieces of information about a NodeSet's reservation:
//
//   - Whether Slurm-operator has created the reservation in it's current configuration as defined in the NodeSet spec,
//     regardless of whether or not that reservation has since been reaped by Slurm or deleted by a user. This is crucial
//     because reservationExists alone is insufficient to determine whether or not a one-off reservation has been created,
//     executed to completion, and was reaped by slurm - resulting in errors if we attempt to recreate based on
//     reservationExists alone. This piece of information is conveyed by the presence of the condition.
//
//   - Whether the reservation presently defined in the NodeSet spec exists in the Slurm cluster. This satisfies the
//     following cases:
//
//   - If a reservation was created by Slurm-operator per the spec and was reaped by Slurm after completion, we must not
//     attempt to recreate it. In order to convey this state, the Condition will be present on a NodeSet with
//     status = true and LastTransitionTime will match the start time defined in the NodeSet spec.
//
//   - If a reoccuring reservation was created by Slurm-operator per the spec, the first occurrence of the reservation completes,
//     and the start time of the reservation was then updated by Slurm to the next iteration, we must respect the start time
//     set by Slurm. In this case, we must not attempt to re-create the reservation with the original time in the NodeSet spec
//     and must instead confirm that the original start time of the reoccuring reservation matches what was set in our NodeSet
//     spec. In order to convey this state, the Condition will be present on a NodeSet with status = true and LastTransitionTime
//     will match the start time defined in the NodeSet spec.
//
//   - If a reoccuring or one-off reservation was created by Slurm-operator per the spec, and the user updates the
//     reservation's start time in the NodeSet spec, we must attempt to re-create the reservation. To convey this state,
//     the condition will be present on a NodeSet with status = false and LastTransitionTime will match the new start
//     time as defined in the NodeSet spec.
func (r *NodeSetReconciler) calculateReservationCondition(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, conditions []metav1.Condition) (*metav1.Condition, error) {
	reservationExists, err := r.slurmControl.CheckReservationForNodeSet(ctx, nodeset)
	if err != nil {
		return nil, err
	}

	nodesetIsScheduled := nodeset.Spec.UpdateStrategy.Type == slinkyv1beta1.ScheduledUpdateNodeSetStrategyType
	nodeSetHasReplicas := ptr.Deref(nodeset.Spec.Replicas, defaults.DefaultNodeSetReplicas) > 0

	if !nodesetIsScheduled || !nodeSetHasReplicas ||
		(reservationExists && !nodeset.DeletionTimestamp.IsZero()) {
		return nil, nil //nolint:nilnil
	}

	reservationCondition := meta.FindStatusCondition(conditions, slurmconditions.NodeSetConditionReservationCreated)

	if reservationCondition != nil {
		switch {
		// If the condition already exists, but the status is false
		case reservationExists && reservationCondition.Status != metav1.ConditionTrue:
			condition := &metav1.Condition{
				Type:               slurmconditions.NodeSetConditionReservationCreated,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: nodeset.Spec.UpdateStrategy.ScheduledUpdate.StartTime,
				ObservedGeneration: 1,
				Reason:             "Created",
			}
			return condition, nil

		// If the condition already exists, but the reservation start time does not match
		case reservationExists && reservationCondition.LastTransitionTime != nodeset.Spec.UpdateStrategy.ScheduledUpdate.StartTime:
			condition := &metav1.Condition{
				Type:               slurmconditions.NodeSetConditionReservationCreated,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: nodeset.Spec.UpdateStrategy.ScheduledUpdate.StartTime,
				ObservedGeneration: reservationCondition.ObservedGeneration + 1,
				Reason:             "Created",
			}
			return condition, nil

		// If the reservation exists, the condition should still exist
		// so that we can distinguish between a one-off reservation that has not yet been created
		// (that we must create) and one that has been created and reaped by slurm (which we must not create)
		case !reservationExists && reservationCondition.Status == metav1.ConditionTrue:
			condition := &metav1.Condition{
				Type:               slurmconditions.NodeSetConditionReservationCreated,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: nodeset.Spec.UpdateStrategy.ScheduledUpdate.StartTime,
				ObservedGeneration: reservationCondition.ObservedGeneration + 1,
				Reason:             "NotExists",
			}
			return condition, nil
		}
	}

	// If the condition does not exist already
	if reservationExists {
		condition := &metav1.Condition{
			Type:               slurmconditions.NodeSetConditionReservationCreated,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: nodeset.Spec.UpdateStrategy.ScheduledUpdate.StartTime,
			ObservedGeneration: 1,
			Reason:             "Created",
		}
		return condition, nil
	}

	return nil, nil //nolint:nilnil
}

func (r *NodeSetReconciler) applyReservationCondition(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, conditions *[]metav1.Condition) error {
	conds := ptr.Deref(conditions, []metav1.Condition{})

	reservationCondition, err := r.calculateReservationCondition(ctx, nodeset, conds)
	if err != nil {
		return err
	}

	if reservationCondition != nil {
		meta.SetStatusCondition(conditions, *reservationCondition)
	} else {
		meta.RemoveStatusCondition(conditions, slurmconditions.NodeSetConditionReservationCreated)
	}

	return nil
}

// calculateOrdinalToNode builds the ordinal to node pinning map.
// Add a node pin if pod is scheduled and running.
// Clear the node pin if the node was deleted, or the pod template no longer matches the node.
func (r *NodeSetReconciler) calculateOrdinalToNode(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) (map[string]string, error) {
	if !nodeset.Spec.PinToNode || nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeDaemonset {
		return nil, nil //nolint:nilnil
	}

	ordinalToNode := make(map[string]string)
	for ordinalStr, nodeName := range nodeset.Status.OrdinalToNode {
		node := &corev1.Node{}
		nodeKey := types.NamespacedName{Name: nodeName}
		if err := r.Get(ctx, nodeKey, node); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		pod := nodesetutils.NewNodeSetSimulatedPod(r.Client, nodeset, &slinkyv1beta1.Controller{}, nodeName)
		if shouldRun, _ := nodesetutils.PodShouldRunOnNode(ctx, pod, node); !shouldRun {
			continue
		}
		ordinalToNode[ordinalStr] = nodeName
	}

	for _, pod := range pods {
		nodeName := pod.Spec.NodeName
		if nodeName == "" || !podutils.IsRunning(pod) {
			continue
		}
		ordinal := nodesetutils.GetOrdinal(pod)
		if ordinal < 0 {
			continue
		}

		node := &corev1.Node{}
		nodeKey := types.NamespacedName{Name: nodeName}
		if err := r.Get(ctx, nodeKey, node); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		podFromTemplate := nodesetutils.NewNodeSetSimulatedPod(r.Client, nodeset, &slinkyv1beta1.Controller{}, nodeName)
		if shouldRun, _ := nodesetutils.PodShouldRunOnNode(ctx, podFromTemplate, node); !shouldRun {
			continue
		}
		ordinalToNode[strconv.Itoa(ordinal)] = nodeName
	}

	return ordinalToNode, nil
}

// Sync NodeSet Pod Conditions to reflect Slurm base and flag states
func (r *NodeSetReconciler) syncNodeSetPodStatus(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
) error {
	slurmNodeStatus, err := r.slurmControl.CalculateNodeStatus(ctx, nodeset, pods)
	if err != nil {
		return err
	}

	if err := r.updateNodeSetPodConditions(ctx, pods, &slurmNodeStatus); err != nil {
		return err
	}

	if err := r.updateNodeSetPodPDBLabels(ctx, nodeset, pods); err != nil {
		return err
	}

	return nil
}

// updateNodeSetPodConditions will iterate over the base states and flag states and
// set pod conditions on the appropriate NodeSet pod to reflect the Slurm states.
func (r *NodeSetReconciler) updateNodeSetPodConditions(
	ctx context.Context,
	pods []*corev1.Pod,
	nodeStatus *slurmcontrol.SlurmNodeStatus,
) error {
	logger := log.FromContext(ctx)
	for _, pod := range pods {
		mutateFn := func(pod *corev1.Pod) error {
			slurmCond := nodeStatus.NodeStates[nodesetutils.GetSlurmNodeName(pod)]
			applySlurmPodConditions(&pod.Status, slurmCond)
			return nil
		}
		if err := objectutils.StatusPatchObject(r.Client, ctx, pod, mutateFn); err != nil {
			if !apierrors.IsNotFound(err) {
				logger.Error(err, "Error patching pod condition", "pod", klog.KObj(pod))
				return err
			}
		}
	}
	return nil
}

func applySlurmPodConditions(status *corev1.PodStatus, desired []corev1.PodCondition) {
	desiredTypes := make(set.Set[corev1.PodConditionType], len(desired))
	for _, cond := range desired {
		desiredTypes.Insert(cond.Type)
		podutil.UpdatePodCondition(status, &cond)
	}
	filtered := status.Conditions[:0]
	for _, c := range status.Conditions {
		if strings.HasPrefix(string(c.Type), slurmconditions.PodStatePrefix) {
			if !desiredTypes.Has(c.Type) {
				continue // drop stale Slurm condition
			}
		}
		filtered = append(filtered, c)
	}
	status.Conditions = filtered
}

// updateNodeSetStatus handles updating the NodeSet status on the Kubernetes API.
// The Status update will be retried on all failures other than NotFound.
func (r *NodeSetReconciler) updateNodeSetStatus(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	newStatus *slinkyv1beta1.NodeSetStatus,
) error {
	logger := log.FromContext(ctx)

	namespacedName := types.NamespacedName{
		Namespace: nodeset.GetNamespace(),
		Name:      nodeset.GetName(),
	}

	logger.V(1).Info("Pending NodeSet Status update",
		"newStatus", newStatus)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toUpdate := &slinkyv1beta1.NodeSet{}
		if err := r.Get(ctx, namespacedName, toUpdate); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		toUpdate.Status = *newStatus
		return r.Status().Update(ctx, toUpdate)
	})
}

// updateNodeSetPodPDBLabels handles updating the NodeSet labels
func (r *NodeSetReconciler) updateNodeSetPodPDBLabels(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	syncPodPDBLabelsFn := func(i int) error {
		pod := pods[i]
		mutateFn := func(pod *corev1.Pod) error {
			podProtect := slurmconditions.IsNodeBusy(&pod.Status)
			logger.V(1).Info("Pending Pod Label update", "pod", klog.KObj(pod), "podProtect", podProtect)
			if podProtect && ptr.Deref(nodeset.Spec.WorkloadDisruptionProtection, defaults.DefaultNodeSetWorkloadDisruptionProtection) {
				pod.Labels[slinkyv1beta1.LabelNodeSetPodProtect] = "true"
			} else {
				delete(pod.Labels, slinkyv1beta1.LabelNodeSetPodProtect)
			}
			return nil
		}
		if err := objectutils.PatchObject(r.Client, ctx, pod, mutateFn); err != nil {
			if !apierrors.IsNotFound(err) {
				logger.Error(err, "failed to patch pod labels for PDB", "pod", klog.KObj(pod))
				return err
			}
		}
		return nil
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, syncPodPDBLabelsFn); err != nil {
		return err
	}

	return nil
}
