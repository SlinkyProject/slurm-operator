// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginset

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/builder/labels"
)

// syncStatus handles determining and updating the status.
func (r *LoginSetReconciler) syncStatus(
	ctx context.Context,
	loginset *slinkyv1beta1.LoginSet,
) error {
	logger := log.FromContext(ctx)

	selectorLabels := labels.NewBuilder().WithLoginSelectorLabels(loginset).Build()
	selector := k8slabels.SelectorFromSet(k8slabels.Set(selectorLabels))

	replicaStatus, err := r.calculateReplicaStatus(ctx, loginset)
	if err != nil {
		return err
	}

	newStatus := &slinkyv1beta1.LoginSetStatus{
		Replicas:   replicaStatus.Replicas,
		Selector:   selector.String(),
		Conditions: []metav1.Condition{},
	}
	newStatus.Conditions = append(newStatus.Conditions, loginset.Status.Conditions...)

	if apiequality.Semantic.DeepEqual(loginset.Status, newStatus) {
		logger.V(2).Info("LoginSet Status has not changed, skipping status update",
			"loginset", klog.KObj(loginset), "status", loginset.Status)
		return nil
	}

	if err := r.updateStatus(ctx, loginset, newStatus); err != nil {
		return fmt.Errorf("error updating LoginSet(%s) status: %w",
			klog.KObj(loginset), err)
	}

	return nil
}

type replicaStatus struct {
	Replicas int32
}

// calculateReplicaStatus will calculate the status of the given pods.
func (r *LoginSetReconciler) calculateReplicaStatus(
	ctx context.Context,
	loginset *slinkyv1beta1.LoginSet,
) (replicaStatus, error) {
	deployment := &appsv1.Deployment{}
	deploymentKey := loginset.Key()
	if err := r.Get(ctx, deploymentKey, deployment); err != nil {
		return replicaStatus{}, err
	}

	status := replicaStatus{
		Replicas: deployment.Status.Replicas,
	}

	return status, nil
}

func (r *LoginSetReconciler) updateStatus(
	ctx context.Context,
	cluster *slinkyv1beta1.LoginSet,
	newStatus *slinkyv1beta1.LoginSetStatus,
) error {
	logger := log.FromContext(ctx)

	namespacedName := types.NamespacedName{
		Namespace: cluster.GetNamespace(),
		Name:      cluster.GetName(),
	}

	logger.V(1).Info("Pending LoginSet Status update",
		"cluster", klog.KObj(cluster), "newStatus", newStatus)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toUpdate := &slinkyv1beta1.LoginSet{}
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
