// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginset

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
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
)

var _ handler.EventHandler = &controllerEventHandler{}

type controllerEventHandler struct {
	client.Reader
	refResolver *refresolver.RefResolver
}

func (e *controllerEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *controllerEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.ObjectNew, q)
}

func (e *controllerEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *controllerEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

func (e *controllerEventHandler) enqueueRequest(
	ctx context.Context,
	obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	controller, ok := obj.(*slinkyv1beta1.Controller)
	if !ok {
		return
	}

	list, err := e.refResolver.GetLoginSetsForController(ctx, controller)
	if err != nil {
		logger.Error(err, "failed to list LoginSets referencing Controller")
		return
	}

	for _, item := range list.Items {
		objectutils.EnqueueRequest(q, &item)
	}
}

var _ handler.EventHandler = &secretEventHandler{}

type secretEventHandler struct {
	client.Reader
	refResolver *refresolver.RefResolver
}

func (e *secretEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *secretEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.ObjectNew, q)
}

func (e *secretEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *secretEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

func (e *secretEventHandler) enqueueRequest(
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

		loginsetList, err := e.refResolver.GetLoginSetsForController(ctx, &controller)
		if err != nil {
			logger.Error(err, "failed to list LoginSet CRs")
		}

		for _, loginset := range loginsetList.Items {
			key := client.ObjectKeyFromObject(&controller)
			if loginset.Spec.ControllerRef.IsMatch(key) {
				objectutils.EnqueueRequest(q, &loginset)
			}
		}
	}
}
