// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/builder/labels"
	"github.com/togethercomputer/slurm-operator/internal/controller/nodeset/slurmcontrol"
	nodesetutils "github.com/togethercomputer/slurm-operator/internal/controller/nodeset/utils"
	"github.com/togethercomputer/slurm-operator/internal/utils"
	"github.com/togethercomputer/slurm-operator/internal/utils/historycontrol"
	"github.com/togethercomputer/slurm-operator/internal/utils/mathutils"
	"github.com/togethercomputer/slurm-operator/internal/utils/podutils"
	"github.com/togethercomputer/slurm-operator/internal/utils/structutils"
	slurmconditions "github.com/togethercomputer/slurm-operator/pkg/conditions"
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

	replicaStatus := r.calculateReplicaStatus(nodeset, pods, currentRevision, updateRevision)
	slurmNodeStatus, err := r.slurmControl.CalculateNodeStatus(ctx, nodeset, pods)
	if err != nil {
		return err
	}

	newStatus := &slinkyv1beta1.NodeSetStatus{
		Replicas:            replicaStatus.Replicas,
		UpdatedReplicas:     replicaStatus.Updated,
		ReadyReplicas:       replicaStatus.Ready,
		AvailableReplicas:   replicaStatus.Available,
		UnavailableReplicas: replicaStatus.Unavailable,
		SlurmIdle:           slurmNodeStatus.Idle,
		SlurmAllocated:      slurmNodeStatus.Allocated + slurmNodeStatus.Mixed,
		SlurmDown:           slurmNodeStatus.Down,
		SlurmDrain:          slurmNodeStatus.Drain,
		ObservedGeneration:  nodeset.Generation,
		NodeSetHash:         hash,
		CollisionCount:      &collisionCount,
		Selector:            selector.String(),
		Conditions:          []metav1.Condition{},
	}
	newStatus.Conditions = append(newStatus.Conditions, nodeset.Status.Conditions...)

	if apiequality.Semantic.DeepEqual(nodeset.Status, newStatus) {
		logger.V(2).Info("NodeSet Status has not changed, skipping status update", "status", nodeset.Status)
		return nil
	}

	if err := r.updateNodeSetStatus(ctx, nodeset, newStatus); err != nil {
		return err
	}

	key := klog.KObj(nodeset).String()
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
}

// calculateReplicaStatus will calculate the status of the given pods.
func (r *NodeSetReconciler) calculateReplicaStatus(
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
	currentRevision, updateRevision *appsv1.ControllerRevision,
) replicaStatus {
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
	// Infer the Unavailable replicas
	status.Unavailable = mathutils.Clamp(status.Replicas-status.Available, 0, status.Replicas)

	return status
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

	if err := r.updateNodeSetPodPDBLabels(ctx, pods); err != nil {
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
		toUpdate := pod.DeepCopy()

		podConditions := nodeStatus.NodeStates[nodesetutils.GetNodeName(toUpdate)]

		// Filter previous SlurmNodeStates that are no longer present
		var filteredConditions []corev1.PodCondition
		for _, condition := range toUpdate.Status.Conditions {
			// Keep any conditions that is not a SlurmNodeState
			if !strings.HasPrefix(string(condition.Type), slurmconditions.StatePrefix) {
				filteredConditions = append(filteredConditions, condition)
			} else {
				// Keep SlurmNodeStates that are still present
				for _, cond := range podConditions {
					_, c := podutil.GetPodCondition(&pod.Status, cond.Type)
					if c != nil {
						filteredConditions = append(filteredConditions, *c)
					}
				}
			}
		}
		toUpdate.Status.Conditions = filteredConditions

		// Add current Slurm node base and flag states
		var condChanged bool
		for _, cond := range podConditions {
			if podutil.UpdatePodCondition(&toUpdate.Status, &cond) && !condChanged {
				condChanged = true
			}
		}
		err := r.Status().Patch(ctx, toUpdate, client.StrategicMergeFrom(pod))
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			logger.Error(err, "Error patching pod condition", "pod", klog.KObj(toUpdate))
			return err
		}
	}
	return nil
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
	pods []*corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	syncPodPDBLabelsFn := func(i int) error {
		pod := pods[i]

		// Refresh Pod to get current Pod Conditions
		podKey := client.ObjectKeyFromObject(pod)
		if err := r.Get(ctx, podKey, pod); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		podProtect := slurmconditions.IsNodeBusy(&pod.Status)
		logger.V(1).Info("Pending Pod Label update", "pod", klog.KObj(pod), "podProtect", podProtect)
		toUpdate := pod.DeepCopy()

		if podProtect {
			podLabel := labels.NewBuilder().WithPodProtect().Build()
			toUpdate.Labels = structutils.MergeMaps(toUpdate.Labels, podLabel)
		} else {
			delete(toUpdate.Labels, slinkyv1beta1.LabelNodeSetPodProtect)
		}

		if err := r.Patch(ctx, toUpdate, client.StrategicMergeFrom(pod)); err != nil {
			logger.Error(err, "failed to patch pod labels for PDB", "pod", klog.KObj(toUpdate))
			return err
		}
		return r.Patch(ctx, toUpdate, client.StrategicMergeFrom(pod))
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, syncPodPDBLabelsFn); err != nil {
		return err
	}

	return nil
}
