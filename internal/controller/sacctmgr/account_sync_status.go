// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package sacctmgr

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/sacctmgr/slurmcontrol"
)

// syncStatus updates the Account's Ready condition based on the apply result.
func (r *AccountReconciler) syncStatus(ctx context.Context, account *slinkyv1beta1.Account, applyErr error) error {
	cond := metav1.Condition{
		Type:               ConditionReady,
		ObservedGeneration: account.Generation,
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
		cond.Message = "Account synced to Slurm"
	}

	meta.SetStatusCondition(&account.Status.Conditions, cond)

	key := types.NamespacedName{Namespace: account.Namespace, Name: account.Name}
	conds := account.Status.Conditions
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toUpdate := &slinkyv1beta1.Account{}
		if err := r.Get(ctx, key, toUpdate); err != nil {
			return client.IgnoreNotFound(err)
		}
		toUpdate.Status.Conditions = conds
		return r.Status().Update(ctx, toUpdate)
	})
}
