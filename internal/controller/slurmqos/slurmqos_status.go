// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmqos

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

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

const (
	ConditionTypeSynced = "Synced"
)

func (r *SlurmQosReconciler) syncStatus(
	ctx context.Context,
	qos *slinkyv1beta1.SlurmQos,
	ready bool,
	id *int32,
	message string,
) error {
	logger := log.FromContext(ctx)

	newStatus := qos.Status.DeepCopy()
	newStatus.Ready = ready
	newStatus.ID = id

	newConditionStatus := metav1.ConditionTrue
	reason := "Synced"
	msg := "QOS is synced with Slurm"
	if !ready {
		newConditionStatus = metav1.ConditionFalse
		reason = "SyncFailed"
		msg = message
	}

	// Preserve LastTransitionTime when the condition status hasn't changed
	now := metav1.Now()
	transitionTime := now
	for _, c := range newStatus.Conditions {
		if c.Type == ConditionTypeSynced && c.Status == newConditionStatus {
			transitionTime = c.LastTransitionTime
			break
		}
	}

	condition := metav1.Condition{
		Type:               ConditionTypeSynced,
		Status:             newConditionStatus,
		ObservedGeneration: qos.Generation,
		LastTransitionTime: transitionTime,
		Reason:             reason,
		Message:            msg,
	}

	replaced := false
	for i, c := range newStatus.Conditions {
		if c.Type == ConditionTypeSynced {
			newStatus.Conditions[i] = condition
			replaced = true
			break
		}
	}
	if !replaced {
		newStatus.Conditions = append(newStatus.Conditions, condition)
	}

	if apiequality.Semantic.DeepEqual(qos.Status, *newStatus) {
		logger.V(2).Info("SlurmQos Status has not changed, skipping status update",
			"slurmqos", klog.KObj(qos), "status", qos.Status)
		return nil
	}

	if err := r.updateStatus(ctx, qos, newStatus); err != nil {
		return fmt.Errorf("error updating SlurmQos(%s) status: %w",
			klog.KObj(qos), err)
	}

	return nil
}

func (r *SlurmQosReconciler) updateStatus(
	ctx context.Context,
	qos *slinkyv1beta1.SlurmQos,
	newStatus *slinkyv1beta1.SlurmQosStatus,
) error {
	logger := log.FromContext(ctx)

	namespacedName := types.NamespacedName{
		Namespace: qos.GetNamespace(),
		Name:      qos.GetName(),
	}

	logger.V(1).Info("Pending SlurmQos Status update",
		"slurmqos", klog.KObj(qos), "newStatus", newStatus)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toUpdate := &slinkyv1beta1.SlurmQos{}
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
