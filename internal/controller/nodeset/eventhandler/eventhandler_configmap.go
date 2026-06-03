// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/indexes"
)

func NewConfigMapEventHandler(reader client.Reader) *ConfigMapEventHandler {
	return &ConfigMapEventHandler{Reader: reader}
}

var _ handler.EventHandler = &ConfigMapEventHandler{}

type ConfigMapEventHandler struct {
	client.Reader
}

func (e *ConfigMapEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *ConfigMapEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.ObjectNew, q)
}

func (e *ConfigMapEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *ConfigMapEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

func (e *ConfigMapEventHandler) enqueueRequest(
	ctx context.Context,
	obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return
	}
	enqueueNodeSetsForConfigRef(ctx, e.Reader, q, cm.GetNamespace(), indexes.ConfigMapRefPrefix+cm.GetName())
}
