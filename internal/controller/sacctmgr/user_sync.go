// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package sacctmgr

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
)

// Sync reconciles a single User to its desired state in Slurm. A User is only
// applied once all the accounts it references exist in Slurm.
func (r *UserReconciler) Sync(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	user := &slinkyv1beta1.User{}
	if err := r.Get(ctx, req.NamespacedName, user); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	user = user.DeepCopy()
	defaults.SetUserDefaults(user)

	if !user.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(user, UserFinalizer) {
			if err := r.control.DeleteUser(ctx, user); err != nil {
				return reconcile.Result{}, err
			}
			controllerutil.RemoveFinalizer(user, UserFinalizer)
			return reconcile.Result{}, r.Update(ctx, user)
		}
		return reconcile.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(user, UserFinalizer) {
		controllerutil.AddFinalizer(user, UserFinalizer)
		if err := r.Update(ctx, user); err != nil {
			return reconcile.Result{}, err
		}
	}

	for _, ua := range user.Spec.Associations {
		exists, err := r.control.AccountExists(ctx, user, ua.Account)
		if err != nil {
			_ = r.syncStatusFailed(ctx, user, err)
			return reconcile.Result{}, err
		}
		if !exists {
			if statusErr := r.syncStatusAccountNotReady(ctx, user, ua.Account); statusErr != nil {
				return reconcile.Result{}, statusErr
			}
			return reconcile.Result{RequeueAfter: userAccountRetry}, nil
		}
	}

	applyErr := r.control.ApplyUser(ctx, user)
	if statusErr := r.syncStatus(ctx, user, applyErr); statusErr != nil {
		return reconcile.Result{}, statusErr
	}
	return reconcile.Result{}, applyErr
}
