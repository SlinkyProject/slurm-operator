// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
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
		h.updateNode(ctx, newNode, q)
	}
}

func (h *nodeEventHandler) updateNode(
	ctx context.Context,
	node *corev1.Node,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	if node.Spec.Unschedulable {
		logger.Info("Node cordoned - triggering workload coordination", "node", node.Name)
	} else {
		logger.Info("Node uncordoned - triggering workload coordination", "node", node.Name)
	}
	h.enqueueNodeSetsForNode(ctx, node, q)
}

// enqueueNodeSetsForNode finds all NodeSets with pods on this node and enqueues them for reconciliation
func (h *nodeEventHandler) enqueueNodeSetsForNode(
	ctx context.Context,
	node *corev1.Node,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	// Find all pods on this node using existing repository pattern
	podList := &corev1.PodList{}
	opts := &client.ListOptions{
		LabelSelector: k8slabels.Everything(),
	}

	if err := h.List(ctx, podList, opts); err != nil {
		logger.Error(err, "Failed to list pods", "node", node.Name)
		return
	}

	// Filter pods on this specific node and enqueue unique NodeSets for reconciliation
	nodesetNames := make(map[string]bool)
	podsOnNode := 0
	for _, pod := range podList.Items {
		if pod.Spec.NodeName != node.Name {
			continue
		}
		podsOnNode++
		if nodesetName := nodesetutils.GetParentName(&pod); nodesetName != "" {
			if !nodesetNames[nodesetName] {
				nodesetNames[nodesetName] = true
				logger.V(1).Info("Enqueueing NodeSet for infrastructure coordination",
					"nodeset", nodesetName, "node", node.Name, "pod", pod.Name)
				q.Add(reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      nodesetName,
						Namespace: pod.Namespace,
					},
				})
			}
		}
	}

	if len(nodesetNames) > 0 {
		logger.V(1).Info("Enqueued NodeSets for infrastructure coordination",
			"node", node.Name, "nodesets", len(nodesetNames), "pods", podsOnNode)
	}
}
