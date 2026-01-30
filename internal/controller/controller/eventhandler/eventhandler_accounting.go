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

func NewAccountingEventHandler(reader client.Reader) *AccountingEventHandler {
	return &AccountingEventHandler{
		Reader:      reader,
		refResolver: refresolver.New(reader),
	}
}

var _ handler.EventHandler = &AccountingEventHandler{}

type AccountingEventHandler struct {
	client.Reader
	refResolver *refresolver.RefResolver
}

func (e *AccountingEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *AccountingEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.ObjectOld, q)
	e.enqueueRequest(ctx, evt.ObjectNew, q)
}

func (e *AccountingEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *AccountingEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

func (e *AccountingEventHandler) enqueueRequest(
	ctx context.Context,
	obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	accounting, ok := obj.(*slinkyv1beta1.Accounting)
	if !ok {
		return
	}

	list, err := e.refResolver.GetControllersForAccounting(ctx, accounting)
	if err != nil {
		logger.Error(err, "failed to list Controllers referencing Accounting")
		return
	}

	for _, item := range list.Items {
		objectutils.EnqueueRequest(q, &item)
	}
}
