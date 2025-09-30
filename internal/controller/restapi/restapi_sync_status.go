// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

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

// syncStatus handles determining and updating the status.
func (r *RestapiReconciler) syncStatus(
	ctx context.Context,
	restapi *slinkyv1beta1.RestApi,
) error {
	logger := log.FromContext(ctx)

	newStatus := &slinkyv1beta1.RestApiStatus{
		Conditions: []metav1.Condition{},
	}
	newStatus.Conditions = append(newStatus.Conditions, restapi.Status.Conditions...)

	if apiequality.Semantic.DeepEqual(restapi.Status, newStatus) {
		logger.V(2).Info("Restapi Status has not changed, skipping status update",
			"restapi", klog.KObj(restapi), "status", restapi.Status)
		return nil
	}

	if err := r.updateStatus(ctx, restapi, newStatus); err != nil {
		return fmt.Errorf("error updating Restapi(%s) status: %w",
			klog.KObj(restapi), err)
	}

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
