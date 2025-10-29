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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

func NewSecretEventHandler(reader client.Reader) *SecretEventHandler {
	return &SecretEventHandler{
		Reader: reader,
	}
}

var _ handler.EventHandler = &SecretEventHandler{}

type SecretEventHandler struct {
	client.Reader
}

func (e *SecretEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *SecretEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.ObjectNew, q)
}

func (e *SecretEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *SecretEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

func (e *SecretEventHandler) enqueueRequest(
	ctx context.Context,
	obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return
	}
	secretKey := client.ObjectKeyFromObject(secret)

	controllerList := &slinkyv1beta1.ControllerList{}
	if err := e.List(ctx, controllerList); err != nil {
		logger.Error(err, "failed to list controller CRs")
	}

	for _, controller := range controllerList.Items {
		slurmKeyKey := controller.AuthSlurmKey()
		jwtHs256KeyKey := controller.AuthJwtHs256Key()
		if secretKey.String() != slurmKeyKey.String() &&
			secretKey.String() != jwtHs256KeyKey.String() {
			continue
		}

		objectutils.EnqueueRequest(q, &controller)
	}
}
