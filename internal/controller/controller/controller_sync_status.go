// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controller

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
func (r *ControllerReconciler) syncStatus(
	ctx context.Context,
	controller *slinkyv1beta1.Controller,
) error {
	logger := log.FromContext(ctx)

	newStatus := &slinkyv1beta1.ControllerStatus{
		Conditions: []metav1.Condition{},
	}
	newStatus.Conditions = append(newStatus.Conditions, controller.Status.Conditions...)

	if apiequality.Semantic.DeepEqual(controller.Status, newStatus) {
		logger.V(2).Info("Controller Status has not changed, skipping status update",
			"controller", klog.KObj(controller), "status", controller.Status)
		return nil
	}

	if err := r.updateStatus(ctx, controller, newStatus); err != nil {
		return fmt.Errorf("error updating Controller(%s) status: %w",
			klog.KObj(controller), err)
	}

	return nil
}

func (r *ControllerReconciler) updateStatus(
	ctx context.Context,
	controller *slinkyv1beta1.Controller,
	newStatus *slinkyv1beta1.ControllerStatus,
) error {
	logger := log.FromContext(ctx)

	namespacedName := types.NamespacedName{
		Namespace: controller.GetNamespace(),
		Name:      controller.GetName(),
	}

	logger.V(1).Info("Pending Controller Status update",
		"controller", klog.KObj(controller), "newStatus", newStatus)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toUpdate := &slinkyv1beta1.Controller{}
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
