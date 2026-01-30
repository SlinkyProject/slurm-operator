// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
)

// syncStatus handles determining and updating the status.
func (r *AccountingReconciler) syncStatus(
	ctx context.Context,
	accounting *slinkyv1beta1.Accounting,
) error {
	logger := log.FromContext(ctx)

	newStatus := &slinkyv1beta1.AccountingStatus{
		Conditions: []metav1.Condition{},
	}
	newStatus.Conditions = append(newStatus.Conditions, accounting.Status.Conditions...)

	if apiequality.Semantic.DeepEqual(accounting.Status, newStatus) {
		logger.V(2).Info("Accounting Status has not changed, skipping status update",
			"accounting", klog.KObj(accounting), "status", accounting.Status)
		return nil
	}

	if err := r.updateStatus(ctx, accounting, newStatus); err != nil {
		return fmt.Errorf("error updating Accounting(%s) status: %w",
			klog.KObj(accounting), err)
	}

	return nil
}

func (r *AccountingReconciler) updateStatus(
	ctx context.Context,
	cluster *slinkyv1beta1.Accounting,
	newStatus *slinkyv1beta1.AccountingStatus,
) error {
	logger := log.FromContext(ctx)

	namespacedName := types.NamespacedName{
		Namespace: cluster.GetNamespace(),
		Name:      cluster.GetName(),
	}

	logger.V(1).Info("Pending Accounting Status update",
		"cluster", klog.KObj(cluster), "newStatus", newStatus)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toUpdate := &slinkyv1beta1.Accounting{}
		if err := r.Get(ctx, namespacedName, toUpdate); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		toUpdate.Status = *newStatus
		return r.Status().Update(ctx, toUpdate)
	})
}
