// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

func pendingNodeRecordRefreshes(nodeset *slinkyv1beta1.NodeSet) set.Set[string] {
	pending := set.New[string]()
	for nodeName := range strings.SplitSeq(nodeset.Annotations[slinkyv1beta1.AnnotationNodeSetPendingNodeRecordRefresh], ",") {
		if nodeName != "" {
			pending.Insert(nodeName)
		}
	}
	return pending
}

func setPendingNodeRecordRefreshes(nodeset *slinkyv1beta1.NodeSet, pending set.Set[string]) {
	if nodeset.Annotations == nil {
		nodeset.Annotations = make(map[string]string)
	}
	if pending.Len() == 0 {
		delete(nodeset.Annotations, slinkyv1beta1.AnnotationNodeSetPendingNodeRecordRefresh)
		return
	}
	nodeset.Annotations[slinkyv1beta1.AnnotationNodeSetPendingNodeRecordRefresh] = strings.Join(pending.SortedList(), ",")
}

// markPendingNodeRecordRefreshes records nodes whose registration-derived
// configuration changed. The marker survives pod deletion and operator
// restarts, allowing the controller to remove the stale Slurm record before it
// creates the replacement pod.
func (r *NodeSetReconciler) markPendingNodeRecordRefreshes(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) error {
	pending := pendingNodeRecordRefreshes(nodeset)
	before := pending.Clone()

	for _, pod := range pods {
		desired, err := r.newReplacementPod(ctx, nodeset, pod, hash)
		if err != nil {
			return err
		}
		currentHash := pod.Annotations[slinkyv1beta1.AnnotationPodRegistrationHash]
		desiredHash := desired.Annotations[slinkyv1beta1.AnnotationPodRegistrationHash]
		nodeName := nodesetutils.GetSlurmNodeName(pod)
		if currentHash != desiredHash {
			pending.Insert(nodeName)
		} else {
			pending.Delete(nodeName)
		}
	}

	if pending.Equal(before) {
		return nil
	}
	return objectutils.PatchObject(r.Client, ctx, nodeset, func(nodeset *slinkyv1beta1.NodeSet) error {
		setPendingNodeRecordRefreshes(nodeset, pending)
		return nil
	})
}

func (r *NodeSetReconciler) newReplacementPod(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pod *corev1.Pod,
	hash string,
) (*corev1.Pod, error) {
	if nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeDaemonset {
		return r.newNodeSetPodDaemon(r.Client, ctx, nodeset, pod.Spec.NodeName, hash)
	}
	return r.newNodeSetPodOrdinal(r.Client, ctx, nodeset, nodesetutils.GetOrdinal(pod), hash)
}

// syncPendingNodeRecordRefreshes waits until the old pod is absent or terminal,
// proving that its slurmd can no longer re-register, then removes the stale
// dynamic Slurm node record. NodeSet pod creation runs only after this step
// succeeds.
func (r *NodeSetReconciler) syncPendingNodeRecordRefreshes(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	pods []*corev1.Pod,
) error {
	pending := pendingNodeRecordRefreshes(nodeset)
	if pending.Len() == 0 {
		return nil
	}
	controllerKey := types.NamespacedName{Namespace: nodeset.Namespace, Name: nodeset.Spec.ControllerRef.Name}
	if r.ClientMap == nil || !r.ClientMap.Has(controllerKey) {
		return fmt.Errorf("cannot refresh dynamic Slurm node records without a client for controller %s/%s",
			nodeset.Namespace, nodeset.Spec.ControllerRef.Name)
	}

	liveNodeNames := set.New[string]()
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil && podutil.IsPodTerminal(pod) {
			continue
		}
		liveNodeNames.Insert(nodesetutils.GetSlurmNodeName(pod))
	}

	logger := log.FromContext(ctx)
	for _, nodeName := range pending.SortedList() {
		if liveNodeNames.Has(nodeName) {
			continue
		}
		logger.Info("Deleting stale dynamic Slurm node record before pod replacement", "node", nodeName)
		if err := r.slurmControl.DeleteNode(ctx, nodeset, nodeName); err != nil {
			return fmt.Errorf("failed to delete stale dynamic Slurm node record %s: %w", nodeName, err)
		}
		pending.Delete(nodeName)
		r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeNormal, SlurmNodeRecordRefreshReason, "Delete",
			"Deleted stale dynamic Slurm node record %s before creating its replacement pod", nodeName)
	}

	if err := objectutils.PatchObject(r.Client, ctx, nodeset, func(nodeset *slinkyv1beta1.NodeSet) error {
		setPendingNodeRecordRefreshes(nodeset, pending)
		return nil
	}); err != nil {
		return err
	}
	klog.FromContext(ctx).V(2).Info("Synchronized pending dynamic Slurm node record refreshes", "remaining", pending.Len())
	return nil
}
