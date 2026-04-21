// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginset

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/SlinkyProject/slurm-operator/internal/syncsteps"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

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

	steps := []syncsteps.Step[*slinkyv1beta1.LoginSet]{
		{
			Name: "SSH Host Keys",
			SyncFn: func(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error {
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
			SyncFn: func(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error {
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
			SyncFn: func(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error {
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
			Name: "Stale Workload",
			SyncFn: func(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error {
				if err := r.cleanupStaleWorkload(ctx, loginset); err != nil {
					return fmt.Errorf("failed to cleanup stale workload: %w", err)
				}
				return nil
			},
		},
		{
			Name: "Workload",
			SyncFn: func(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error {
				object, err := r.builder.BuildLoginWorkload(loginset)
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

	if err := syncsteps.Sync(ctx, r.eventRecorder, loginset, steps); err != nil {
		errs := []error{err}
		if err := r.syncStatus(ctx, loginset); err != nil {
			e := fmt.Errorf("failed status syncFSyncFn: %w", err)
			errs = append(errs, e)
		}
		return utilerrors.NewAggregate(errs)
	}

	return r.syncStatus(ctx, loginset)
}

// cleanupStaleWorkload deletes the previously-rendered workload of the kind
// not selected by Spec.Strategy.Type, so flipping the strategy type does not
// leave a Deployment and a StatefulSet sharing the same name.
func (r *LoginSetReconciler) cleanupStaleWorkload(ctx context.Context, loginset *slinkyv1beta1.LoginSet) error {
	logger := log.FromContext(ctx)
	key := loginset.Key()

	var stale client.Object
	if loginset.Spec.Strategy.Type == slinkyv1beta1.OnDeleteLoginSetStrategyType {
		stale = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}}
	} else {
		stale = &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}}
	}

	if err := r.Get(ctx, key, stale); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get stale workload (%s): %w", klog.KObj(stale), err)
	}

	if !metav1.IsControlledBy(stale, loginset) {
		logger.V(1).Info("Stale workload is not controlled by this LoginSet, skipping",
			"object", klog.KObj(stale))
		return nil
	}

	logger.Info("Deleting stale workload", "object", klog.KObj(stale))
	if err := r.Delete(ctx, stale); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete stale workload (%s): %w", klog.KObj(stale), err)
	}
	return nil
}
