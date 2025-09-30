// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

type SyncStep struct {
	Name string
	Sync func(ctx context.Context, controller *slinkyv1beta1.Controller) error
}

// Sync implements control logic for synchronizing a Controller.
func (r *ControllerReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	controller := &slinkyv1beta1.Controller{}
	if err := r.Get(ctx, req.NamespacedName, controller); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Controller has been deleted", "request", req)
			return nil
		}
		return err
	}

	syncSteps := []SyncStep{
		{
			Name: "Service",
			Sync: func(ctx context.Context, controller *slinkyv1beta1.Controller) error {
				object, err := r.builder.BuildControllerService(controller)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, object, false); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "Config",
			Sync: func(ctx context.Context, controller *slinkyv1beta1.Controller) error {
				object, err := r.builder.BuildControllerConfig(controller)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "StatefulSet",
			Sync: func(ctx context.Context, controller *slinkyv1beta1.Controller) error {
				object, err := r.builder.BuildController(controller)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
	}

	for _, s := range syncSteps {
		if err := s.Sync(ctx, controller); err != nil {
			e := fmt.Errorf("[%s]: %w", s.Name, err)
			errors := []error{e}
			if err := r.syncStatus(ctx, controller); err != nil {
				e := fmt.Errorf("[%s]: %w", s.Name, err)
				errors = append(errors, e)
			}
			return utilerrors.NewAggregate(errors)
		}
	}

	return r.syncStatus(ctx, controller)
}
