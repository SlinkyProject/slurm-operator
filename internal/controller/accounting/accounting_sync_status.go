// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	slurmconditions "github.com/SlinkyProject/slurm-operator/pkg/conditions"
)

// syncStatus handles determining and updating the status.
func (r *AccountingReconciler) syncStatus(
	ctx context.Context,
	accounting *slinkyv1beta1.Accounting,
) error {
	logger := log.FromContext(ctx)

	newStatus := slinkyv1beta1.AccountingStatus{
		Conditions: []metav1.Condition{},
	}
	newStatus.Conditions = append(newStatus.Conditions, accounting.Status.Conditions...)
	if err := r.applyAvailableCondition(ctx, accounting, &newStatus.Conditions); err != nil {
		return err
	}

	if apiequality.Semantic.DeepEqual(accounting.Status, newStatus) {
		logger.V(2).Info("Accounting Status has not changed, skipping status update",
			"accounting", klog.KObj(accounting), "status", accounting.Status)
		return nil
	}

	if err := r.updateStatus(ctx, accounting, &newStatus); err != nil {
		return fmt.Errorf("error updating Accounting(%s) status: %w",
			klog.KObj(accounting), err)
	}

	return nil
}

func (r *AccountingReconciler) applyAvailableCondition(
	ctx context.Context,
	accounting *slinkyv1beta1.Accounting,
	conditions *[]metav1.Condition,
) error {
	if accounting.Spec.External {
		meta.RemoveStatusCondition(conditions, slurmconditions.AccountingConditionAvailable)
		return nil
	}

	condition := metav1.Condition{
		Type:               slurmconditions.AccountingConditionAvailable,
		ObservedGeneration: accounting.Generation,
	}

	statefulset := &appsv1.StatefulSet{}
	err := r.Get(ctx, accounting.Key(), statefulset)
	switch {
	case apierrors.IsNotFound(err):
		condition.Status = metav1.ConditionFalse
		condition.Reason = "StatefulSetNotFound"
		condition.Message = "The slurmdbd StatefulSet does not exist yet"
	case err != nil:
		return err
	case statefulset.Status.ReadyReplicas > 0:
		condition.Status = metav1.ConditionTrue
		condition.Reason = "MinimumReplicasAvailable"
		condition.Message = "The slurmdbd StatefulSet has a ready replica"
	default:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NoReplicasAvailable"
		condition.Message = "The slurmdbd StatefulSet has no ready replicas"
	}

	meta.SetStatusCondition(conditions, condition)

	return nil
}

func (r *AccountingReconciler) updateStatus(
	ctx context.Context,
	accounting *slinkyv1beta1.Accounting,
	newStatus *slinkyv1beta1.AccountingStatus,
) error {
	logger := log.FromContext(ctx)

	namespacedName := types.NamespacedName{
		Namespace: accounting.GetNamespace(),
		Name:      accounting.GetName(),
	}

	logger.V(1).Info("Pending Accounting Status update",
		"accounting", klog.KObj(accounting), "newStatus", newStatus)
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
