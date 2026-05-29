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

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
)

func NewRestApiEventHandler(reader client.Reader) *RestApiEventHandler {
	return &RestApiEventHandler{
		Reader:      reader,
		refResolver: refresolver.New(reader),
	}
}

var _ handler.EventHandler = &RestApiEventHandler{}

type RestApiEventHandler struct {
	client.Reader
	refResolver *refresolver.RefResolver
}

func (e *RestApiEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *RestApiEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.ObjectOld, q)
	e.enqueueRequest(ctx, evt.ObjectNew, q)
}

func (e *RestApiEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *RestApiEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

func (e *RestApiEventHandler) enqueueRequest(
	ctx context.Context,
	obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	restapi, ok := obj.(*slinkyv1beta1.RestApi)
	if !ok {
		return
	}

	controller, err := e.refResolver.GetController(ctx, restapi.Spec.ControllerRef)
	if err != nil {
		logger.Error(err, "failed to Get RestApi referencing Controller")
		return
	}

	objectutils.EnqueueRequest(q, controller)
}
