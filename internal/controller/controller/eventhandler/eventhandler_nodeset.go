// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
)

func NewNodeSetEventHandler(reader client.Reader) *NodesetEventHandler {
	return &NodesetEventHandler{
		Reader:      reader,
		refResolver: refresolver.New(reader),
	}
}

var _ handler.EventHandler = &NodesetEventHandler{}

type NodesetEventHandler struct {
	client.Reader
	refResolver *refresolver.RefResolver
}

// Create implements handler.TypedEventHandler.
func (e *NodesetEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

// Delete implements handler.TypedEventHandler.
func (e *NodesetEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

// Generic implements handler.TypedEventHandler.
func (e *NodesetEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

// Update implements handler.TypedEventHandler.
func (e *NodesetEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.ObjectNew, q)
}

func (e *NodesetEventHandler) enqueueRequest(ctx context.Context, obj client.Object, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	nodeset, ok := obj.(*slinkyv1beta1.NodeSet)
	if !ok {
		return
	}

	controller, err := e.refResolver.GetController(ctx, nodeset.Spec.ControllerRef)
	if err != nil {
		return
	}

	objectutils.EnqueueRequest(q, controller)
}
