// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/SlinkyProject/slurm-operator/internal/syncsteps"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

// Sync implements control logic for synchronizing a Accounting.
func (r *AccountingReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	accounting := &slinkyv1beta1.Accounting{}
	if err := r.Get(ctx, req.NamespacedName, accounting); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Accounting has been deleted", "request", req)
			return nil
		}
		return err
	}
	accounting = accounting.DeepCopy()
	defaults.SetAccountingDefaults(accounting)

	if !accounting.DeletionTimestamp.IsZero() {
		logger.Info("Accounting is being deleted, skipping sync", "request", req)
		return nil
	}

	steps := []syncsteps.Step[*slinkyv1beta1.Accounting]{
		{
			Name: "Service",
			SyncFn: func(ctx context.Context, accounting *slinkyv1beta1.Accounting) error {
				if accounting.Spec.External {
					return nil
				}
				object, err := r.builder.BuildAccountingService(accounting)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, accounting, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "Config",
			SyncFn: func(ctx context.Context, accounting *slinkyv1beta1.Accounting) error {
				if accounting.Spec.External {
					return nil
				}
				object, err := r.builder.BuildAccountingConfig(accounting)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, accounting, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "StatefulSet",
			SyncFn: func(ctx context.Context, accounting *slinkyv1beta1.Accounting) error {
				if accounting.Spec.External {
					return nil
				}
				object, err := r.builder.BuildAccounting(accounting)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, accounting, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
	}

	if err := syncsteps.Sync(ctx, r.eventRecorder, accounting, steps); err != nil {
		errs := []error{err}
		if err := r.syncStatus(ctx, accounting); err != nil {
			e := fmt.Errorf("failed status syncFn: %w", err)
			errs = append(errs, e)
		}
		return utilerrors.NewAggregate(errs)
	}

	return r.syncStatus(ctx, accounting)
}
