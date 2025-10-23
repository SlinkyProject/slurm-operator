// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

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

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

var _ handler.EventHandler = &nodeEventHandler{}

type nodeEventHandler struct {
	client.Reader
}

// Create implements handler.EventHandler
func (h *nodeEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

// Delete implements handler.EventHandler
func (h *nodeEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

// Generic implements handler.EventHandler
func (h *nodeEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

// Update implements handler.EventHandler
func (h *nodeEventHandler) Update(
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

	// Detect node cordoning/uncordoning
	if oldNode.Spec.Unschedulable != newNode.Spec.Unschedulable {
		h.enqueueNodeSetsForNode(ctx, newNode, q)
	}

	// Detect node annotation updates
	if !apiequality.Semantic.DeepEqual(oldNode.Annotations, newNode.Annotations) {
		h.enqueueNodeSetsForNode(ctx, newNode, q)
	}
}

func (h *nodeEventHandler) enqueueNodeSetsForNode(
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

func (h *nodeEventHandler) resolveControllerRef(
	ctx context.Context,
	namespace string,
	controllerRef *metav1.OwnerReference,
) *slinkyv1alpha1.NodeSet {
	if controllerRef.Kind != slinkyv1alpha1.NodeSetKind || controllerRef.APIVersion != slinkyv1alpha1.NodeSetAPIVersion {
		return nil
	}

	nodeset := &slinkyv1alpha1.NodeSet{}
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
