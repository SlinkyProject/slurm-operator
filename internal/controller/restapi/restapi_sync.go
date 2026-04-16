// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

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
	Sync func(ctx context.Context, cluster *slinkyv1beta1.RestApi) error
}

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

	syncSteps := []SyncStep{
		{
			Name: "Service",
			Sync: func(ctx context.Context, restapi *slinkyv1beta1.RestApi) error {
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
			Sync: func(ctx context.Context, restapi *slinkyv1beta1.RestApi) error {
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
		{
			Name: "PodDisruptionBudget",
			Sync: func(ctx context.Context, restapi *slinkyv1beta1.RestApi) error {
				object, err := r.builder.BuildRestapiPodDisruptionBudget(restapi)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if !restapiPodDisruptionBudgetDesired(restapi) {
					if err := objectutils.DeleteObject(r.Client, ctx, r.eventRecorder, restapi, object); err != nil {
						return fmt.Errorf("failed to delete object (%s): %w", klog.KObj(object), err)
					}
					return nil
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, restapi, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
	}

	for _, s := range syncSteps {
		if err := s.Sync(ctx, restapi); err != nil {
			msg := fmt.Sprintf("Failed %q step: %v", s.Name, err)
			r.eventRecorder.Eventf(restapi, nil, corev1.EventTypeWarning, SyncFailedReason, "Sync", msg)
			e := fmt.Errorf("failed %q step: %w", s.Name, err)
			errs := []error{e}
			if err := r.syncStatus(ctx, restapi); err != nil {
				e := fmt.Errorf("failed status sync: %w", err)
				errs = append(errs, e)
			}
			return utilerrors.NewAggregate(errs)
		}
	}

	return r.syncStatus(ctx, restapi)
}
