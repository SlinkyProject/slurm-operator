// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

type SyncStep struct {
	Name string
	Sync func(ctx context.Context, accounting *slinkyv1beta1.Accounting) error
}

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

	syncSteps := []SyncStep{
		{
			Name: "Service",
			Sync: func(ctx context.Context, accounting *slinkyv1beta1.Accounting) error {
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
			Sync: func(ctx context.Context, accounting *slinkyv1beta1.Accounting) error {
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
			Sync: func(ctx context.Context, accounting *slinkyv1beta1.Accounting) error {
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

	for _, s := range syncSteps {
		if err := s.Sync(ctx, accounting); err != nil {
			msg := fmt.Sprintf("Failed %q step: %v", s.Name, err)
			r.eventRecorder.Eventf(accounting, nil, corev1.EventTypeWarning, SyncFailedReason, "Sync", msg)
			e := fmt.Errorf("failed %q step: %w", s.Name, err)
			errs := []error{e}
			if err := r.syncStatus(ctx, accounting); err != nil {
				e := fmt.Errorf("failed status sync: %w", err)
				errs = append(errs, e)
			}
			return utilerrors.NewAggregate(errs)
		}
	}

	return r.syncStatus(ctx, accounting)
}
