// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

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

// Sync implements control logic for synchronizing a Restapi.
func (r *RestapiReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	restapi := &slinkyv1beta1.RestApi{}
	if err := r.Get(ctx, req.NamespacedName, restapi); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Restapi has been deleted", "request", req)
			return nil
		}
		return err
	}
	restapi = restapi.DeepCopy()
	defaults.SetRestApiDefaults(restapi)

	if !restapi.DeletionTimestamp.IsZero() {
		logger.Info("Restapi is being deleted, skipping sync", "request", req)
		return nil
	}

	steps := []syncsteps.Step[*slinkyv1beta1.RestApi]{
		{
			Name: "Service",
			SyncFn: func(ctx context.Context, restapi *slinkyv1beta1.RestApi) error {
				object, err := r.builder.BuildRestapiService(restapi)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, restapi, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "Deployment",
			SyncFn: func(ctx context.Context, restapi *slinkyv1beta1.RestApi) error {
				object, err := r.builder.BuildRestapi(restapi)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, restapi, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
	}

	if err := syncsteps.Sync(ctx, r.eventRecorder, restapi, steps); err != nil {
		errs := []error{err}
		if err := r.syncStatus(ctx, restapi); err != nil {
			e := fmt.Errorf("failed status syncFn: %w", err)
			errs = append(errs, e)
		}
		return utilerrors.NewAggregate(errs)
	}

	return r.syncStatus(ctx, restapi)
}
