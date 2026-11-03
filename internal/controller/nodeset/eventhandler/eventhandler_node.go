// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

func NewNodeEventHandler(reader client.Reader, propagatedNodeConditions []corev1.NodeConditionType) *NodeEventHandler {
	return &NodeEventHandler{
		Reader:                   reader,
		propagatedNodeConditions: propagatedNodeConditions,
	}
}

var _ handler.EventHandler = &NodeEventHandler{}

type NodeEventHandler struct {
	client.Reader

	// propagatedNodeConditions are the Node condition types whose changes
	// affect the Slurm node drain reason.
	propagatedNodeConditions []corev1.NodeConditionType
}

// Create implements handler.EventHandler
func (h *NodeEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

// Delete implements handler.EventHandler
func (h *NodeEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

// Generic implements handler.EventHandler
func (h *NodeEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

// Update implements handler.EventHandler
func (h *NodeEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	oldNode, ok := evt.ObjectOld.(*corev1.Node)
	if !ok {
		return
	}
	newNode, ok := evt.ObjectNew.(*corev1.Node)
	if !ok {
		return
	}

	// Detect node cordoning/uncordoning, metadata changed, or (while cordoned)
	// a change to a condition that feeds the Slurm node drain reason.
	if oldNode.Spec.Unschedulable != newNode.Spec.Unschedulable ||
		!apiequality.Semantic.DeepEqual(oldNode.Annotations, newNode.Annotations) ||
		!apiequality.Semantic.DeepEqual(oldNode.Labels, newNode.Labels) ||
		(newNode.Spec.Unschedulable && h.propagatedConditionsChanged(oldNode, newNode)) {
		h.enqueueNodeSetsForNode(ctx, newNode, q)
	}
}

// propagatedConditionsChanged reports whether any propagated condition was
// added, removed, or changed status, reason, or message. Heartbeat-only
// updates are ignored.
func (h *NodeEventHandler) propagatedConditionsChanged(oldNode, newNode *corev1.Node) bool {
	for _, condType := range h.propagatedNodeConditions {
		if nodeConditionState(oldNode, condType) != nodeConditionState(newNode, condType) {
			return true
		}
	}
	return false
}

type conditionState struct {
	status  corev1.ConditionStatus
	reason  string
	message string
}

// nodeConditionState returns the drain-reason-relevant fields of the node
// condition of the given type, or the zero value if it is absent.
func nodeConditionState(node *corev1.Node, condType corev1.NodeConditionType) conditionState {
	for _, cond := range node.Status.Conditions {
		if cond.Type == condType {
			return conditionState{status: cond.Status, reason: cond.Reason, message: cond.Message}
		}
	}
	return conditionState{}
}

func (h *NodeEventHandler) enqueueNodeSetsForNode(
	ctx context.Context,
	node *corev1.Node,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	podList := &corev1.PodList{}
	opts := []client.ListOption{
		client.MatchingFields{
			"spec.nodeName": node.Name,
		},
	}
	if err := h.List(ctx, podList, opts...); err != nil {
		logger.Error(err, "failed to list pods", "node", node.Name)
		return
	}

	for _, pod := range podList.Items {
		controllerRef := metav1.GetControllerOf(&pod)
		if controllerRef == nil {
			continue
		}
		nodeset := h.resolveControllerRef(ctx, pod.Namespace, controllerRef)
		if nodeset == nil {
			continue
		}
		if node.Spec.Unschedulable {
			logger.Info("Node was cordoned, reconcile NodeSet with Pod on Node",
				"node", node.Name, "nodeset", klog.KObj(nodeset), "pod", klog.KObj(&pod))
		} else {
			logger.Info("Node was uncordoned, reconcile NodeSet with Pod on Node",
				"node", node.Name, "nodeset", klog.KObj(nodeset), "pod", klog.KObj(&pod))
		}
		objectutils.EnqueueRequest(q, nodeset)
	}
}

func (h *NodeEventHandler) resolveControllerRef(
	ctx context.Context,
	namespace string,
	controllerRef *metav1.OwnerReference,
) *slinkyv1beta1.NodeSet {
	if controllerRef.Kind != slinkyv1beta1.NodeSetKind || controllerRef.APIVersion != slinkyv1beta1.NodeSetAPIVersion {
		return nil
	}

	nodeset := &slinkyv1beta1.NodeSet{}
	key := types.NamespacedName{Namespace: namespace, Name: controllerRef.Name}
	if err := h.Get(ctx, key, nodeset); err != nil {
		return nil
	}
	if nodeset.UID != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	return nodeset
}
