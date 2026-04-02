// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginset

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

type SyncStep struct {
	Name string
	Sync func(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error
}

// Sync implements control logic for synchronizing a Cluster.
func (r *LoginSetReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	loginset := &slinkyv1beta1.LoginSet{}
	if err := r.Get(ctx, req.NamespacedName, loginset); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("LoginSet has been deleted", "request", req)
			return nil
		}
		return err
	}
	loginset = loginset.DeepCopy()
	defaults.SetLoginSetDefaults(loginset)

	// Skip reconciliation if the LoginSet is being deleted.
	// Child resources (Deployment, Service, ConfigMap, Secret) have
	// blockOwnerDeletion=true, so the Kubernetes garbage collector will
	// delete them before the LoginSet. If we continue to reconcile, we
	// would recreate children that the GC is trying to delete, causing
	// the LoginSet to be stuck in Terminating state.
	if !loginset.DeletionTimestamp.IsZero() {
		logger.Info("LoginSet is being deleted, skipping sync", "request", req)
		return nil
	}

	controller := &slinkyv1beta1.Controller{}
	controllerKey := client.ObjectKey(loginset.Spec.ControllerRef.NamespacedName())
	if err := r.Get(ctx, controllerKey, controller); err != nil {
		msg := fmt.Sprintf("Failed to get Controller (%s): %v", controllerKey, err)
		r.eventRecorder.Eventf(loginset, nil, corev1.EventTypeWarning, ControllerRefFailedReason, "Sync", msg)
		return fmt.Errorf("failed to get controller (%s): %w", controllerKey, err)
	}

	syncSteps := []SyncStep{
		{
			Name: "SSH Host Keys",
			Sync: func(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error {
				object, err := r.builder.BuildLoginSshHostKeys(loginset)
				if err != nil {
					return fmt.Errorf("failed to build object: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, loginset, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "SSH Config",
			Sync: func(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error {
				object, err := r.builder.BuildLoginSshConfig(loginset)
				if err != nil {
					return fmt.Errorf("failed to build object: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, loginset, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "Service",
			Sync: func(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error {
				object, err := r.builder.BuildLoginService(loginset)
				if err != nil {
					return fmt.Errorf("failed to build object: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, loginset, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "Deployment",
			Sync: func(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error {
				object, err := r.builder.BuildLogin(loginset)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, loginset, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
	}

	for _, s := range syncSteps {
		if err := s.Sync(ctx, loginset); err != nil {
			msg := fmt.Sprintf("Failed %q step: %v", s.Name, err)
			r.eventRecorder.Eventf(loginset, nil, corev1.EventTypeWarning, SyncFailedReason, "Sync", msg)
			e := fmt.Errorf("failed %q step: %w", s.Name, err)
			errs := []error{e}
			if err := r.syncStatus(ctx, loginset); err != nil {
				e := fmt.Errorf("failed status sync: %w", err)
				errs = append(errs, e)
			}
			return utilerrors.NewAggregate(errs)
		}
	}

	return r.syncStatus(ctx, loginset)
}
