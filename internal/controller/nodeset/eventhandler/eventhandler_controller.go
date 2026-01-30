// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/utils/objectutils"
	"github.com/togethercomputer/slurm-operator/internal/utils/refresolver"
)

func NewControllerEventHandler(reader client.Reader) *ControllerEventHandler {
	return &ControllerEventHandler{
		Reader:      reader,
		refResolver: refresolver.New(reader),
	}
}

var _ handler.EventHandler = &ControllerEventHandler{}

type ControllerEventHandler struct {
	client.Reader
	refResolver *refresolver.RefResolver
}

func (e *ControllerEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *ControllerEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.ObjectNew, q)
}

func (e *ControllerEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *ControllerEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

func (e *ControllerEventHandler) enqueueRequest(
	ctx context.Context,
	obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	controller, ok := obj.(*slinkyv1beta1.Controller)
	if !ok {
		return
	}

	list, err := e.refResolver.GetNodeSetsForController(ctx, controller)
	if err != nil {
		logger.Error(err, "failed to list NodeSets referencing Controller")
		return
	}

	for _, item := range list.Items {
		objectutils.EnqueueRequest(q, &item)
	}
}
