// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package sacctmgr

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/sacctmgr/slurmcontrol"
)

// syncStatus updates the User's Ready condition based on the apply result.
func (r *UserReconciler) syncStatus(ctx context.Context, user *slinkyv1beta1.User, applyErr error) error {
	cond := metav1.Condition{
		Type:               ConditionReady,
		ObservedGeneration: user.Generation,
	}
	if applyErr != nil {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "SyncFailed"
		if errors.Is(applyErr, slurmcontrol.ErrNoClient) {
			cond.Reason = "NoClient"
		}
		cond.Message = applyErr.Error()
	} else {
		cond.Status = metav1.ConditionTrue
		cond.Reason = "Synced"
		cond.Message = "User synced to Slurm"
	}

	return r.persistStatus(ctx, user, cond)
}

// syncStatusAccountNotReady sets a not-ready condition while waiting for a
// referenced account to exist in Slurm.
func (r *UserReconciler) syncStatusAccountNotReady(ctx context.Context, user *slinkyv1beta1.User, account string) error {
	cond := metav1.Condition{
		Type:               ConditionReady,
		ObservedGeneration: user.Generation,
		Status:             metav1.ConditionFalse,
		Reason:             "AccountNotReady",
		Message:            fmt.Sprintf("waiting for account %q to exist", account),
	}
	return r.persistStatus(ctx, user, cond)
}

// syncStatusFailed sets a failed condition for a sync error.
func (r *UserReconciler) syncStatusFailed(ctx context.Context, user *slinkyv1beta1.User, err error) error {
	cond := metav1.Condition{
		Type:               ConditionReady,
		ObservedGeneration: user.Generation,
		Status:             metav1.ConditionFalse,
		Reason:             "SyncFailed",
		Message:            err.Error(),
	}
	return r.persistStatus(ctx, user, cond)
}

// persistStatus sets the condition and persists the User status using the same
// conflict-retry Get -> Status().Update pattern as the Account controller.
func (r *UserReconciler) persistStatus(ctx context.Context, user *slinkyv1beta1.User, cond metav1.Condition) error {
	meta.SetStatusCondition(&user.Status.Conditions, cond)

	key := types.NamespacedName{Namespace: user.Namespace, Name: user.Name}
	conds := user.Status.Conditions
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toUpdate := &slinkyv1beta1.User{}
		if err := r.Get(ctx, key, toUpdate); err != nil {
			return client.IgnoreNotFound(err)
		}
		toUpdate.Status.Conditions = conds
		return r.Status().Update(ctx, toUpdate)
	})
}
