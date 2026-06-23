// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package sacctmgr

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
)

// Sync reconciles a single Account to its desired state in Slurm.
func (r *AccountReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	account := &slinkyv1beta1.Account{}
	if err := r.Get(ctx, req.NamespacedName, account); err != nil {
		return client.IgnoreNotFound(err)
	}

	account = account.DeepCopy()
	defaults.SetAccountDefaults(account)

	if !account.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(account, AccountFinalizer) {
			if err := r.control.DeleteAccount(ctx, account); err != nil {
				if r.eventRecorder != nil {
					r.eventRecorder.Eventf(account, nil, corev1.EventTypeWarning, "DeleteFailed", "Delete",
						"failed to delete Account %q from Slurm: %v", account.Spec.AccountName, err)
				}
				return err
			}
			if r.eventRecorder != nil {
				if account.Spec.DeletionPolicy == slinkyv1beta1.DeletionPolicyOrphan {
					r.eventRecorder.Eventf(account, nil, corev1.EventTypeNormal, "Orphaned", "Delete",
						"Account %q orphaned in Slurm (deletionPolicy=Orphan)", account.Spec.AccountName)
				} else {
					r.eventRecorder.Eventf(account, nil, corev1.EventTypeNormal, "Deleted", "Delete",
						"Account %q deleted from Slurm", account.Spec.AccountName)
				}
			}
			controllerutil.RemoveFinalizer(account, AccountFinalizer)
			return r.Update(ctx, account)
		}
		return nil
	}

	if !controllerutil.ContainsFinalizer(account, AccountFinalizer) {
		controllerutil.AddFinalizer(account, AccountFinalizer)
		if err := r.Update(ctx, account); err != nil {
			return err
		}
	}

	applyErr := r.control.ApplyAccount(ctx, account)
	if statusErr := r.syncStatus(ctx, account, applyErr); statusErr != nil {
		return statusErr
	}
	return applyErr
}
