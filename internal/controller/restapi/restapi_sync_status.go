// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

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
func (r *RestapiReconciler) syncStatus(
	ctx context.Context,
	restapi *slinkyv1beta1.RestApi,
) error {
	logger := log.FromContext(ctx)

	newStatus := slinkyv1beta1.RestApiStatus{
		Conditions: []metav1.Condition{},
	}
	newStatus.Conditions = append(newStatus.Conditions, restapi.Status.Conditions...)
	if err := r.applyAvailableCondition(ctx, restapi, &newStatus.Conditions); err != nil {
		return err
	}

	if apiequality.Semantic.DeepEqual(restapi.Status, newStatus) {
		logger.V(2).Info("Restapi Status has not changed, skipping status update",
			"restapi", klog.KObj(restapi), "status", restapi.Status)
		return nil
	}

	if err := r.updateStatus(ctx, restapi, &newStatus); err != nil {
		return fmt.Errorf("error updating Restapi(%s) status: %w",
			klog.KObj(restapi), err)
	}

	return nil
}

func (r *RestapiReconciler) applyAvailableCondition(
	ctx context.Context,
	restapi *slinkyv1beta1.RestApi,
	conditions *[]metav1.Condition,
) error {
	condition := metav1.Condition{
		Type:               slurmconditions.RestApiConditionAvailable,
		ObservedGeneration: restapi.Generation,
	}

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, restapi.Key(), deployment)
	switch {
	case apierrors.IsNotFound(err):
		condition.Status = metav1.ConditionFalse
		condition.Reason = "DeploymentNotFound"
		condition.Message = "The slurmrestd Deployment does not exist yet"
	case err != nil:
		return err
	case deployment.Status.AvailableReplicas > 0:
		condition.Status = metav1.ConditionTrue
		condition.Reason = "MinimumReplicasAvailable"
		condition.Message = "The slurmrestd Deployment has an available replica"
	default:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "NoReplicasAvailable"
		condition.Message = "The slurmrestd Deployment has no available replicas"
	}

	meta.SetStatusCondition(conditions, condition)

	return nil
}

func (r *RestapiReconciler) updateStatus(
	ctx context.Context,
	cluster *slinkyv1beta1.RestApi,
	newStatus *slinkyv1beta1.RestApiStatus,
) error {
	logger := log.FromContext(ctx)

	namespacedName := types.NamespacedName{
		Namespace: cluster.GetNamespace(),
		Name:      cluster.GetName(),
	}

	logger.V(1).Info("Pending Restapi Status update",
		"cluster", klog.KObj(cluster), "newStatus", newStatus)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toUpdate := &slinkyv1beta1.RestApi{}
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
