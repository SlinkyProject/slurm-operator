// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/controller/controller/slurmcontrol"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

// syncStatus handles determining and updating the status.
func (r *ControllerReconciler) syncStatus(
	ctx context.Context,
	controller *slinkyv1beta1.Controller,
	errors ...error,
) error {
	if err := r.syncControllerStatus(ctx, controller); err != nil {
		errors = append(errors, err)
	}

	if err := r.syncHAStatus(ctx, controller); err != nil {
		errors = append(errors, err)
	}

	return utilerrors.NewAggregate(errors)
}

func (r *ControllerReconciler) syncControllerStatus(
	ctx context.Context,
	controller *slinkyv1beta1.Controller,
) error {
	logger := log.FromContext(ctx)

	newStatus := slinkyv1beta1.ControllerStatus{
		Conditions: []metav1.Condition{},
	}
	newStatus.Conditions = append(newStatus.Conditions, controller.Status.Conditions...)

	if apiequality.Semantic.DeepEqual(controller.Status, newStatus) {
		logger.V(2).Info("Controller Status has not changed, skipping status update",
			"controller", klog.KObj(controller), "status", controller.Status)
		return nil
	}

	if err := r.updateStatus(ctx, controller, &newStatus); err != nil {
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

func (r *ControllerReconciler) syncHAStatus(
	ctx context.Context,
	controller *slinkyv1beta1.Controller,
) error {
	if controller.Spec.External {
		return nil
	}

	pings, err := r.slurmControl.GetActiveHAController(ctx, controller)
	if err != nil {
		// Do not bail out when Slurm cannot be asked which controller is
		// active. The Controller Service selects pods by LabelControllerActive,
		// so returning here leaves the Service without endpoints — and a
		// Service without endpoints is exactly what makes Slurm unreachable,
		// so every later reconcile fails the same way and no pod is ever
		// labeled again. The deadlock survives operator restarts because the
		// Slurm client is created once and kept, so the call keeps failing
		// with a transport error rather than ErrNoSlurmClient. Treat any
		// failure like a missing client: fall through with no pings, label the
		// primary, and let a later reconcile move the label once Slurm answers.
		if !errors.Is(err, slurmcontrol.ErrNoSlurmClient) {
			log.FromContext(ctx).V(1).Info("Failed to determine the active controller, defaulting to the primary",
				"Controller", klog.KObj(controller), "error", err)
		}
		pings = nil
	}

	activePodName := controller.PodName(0)
	for i, ping := range pings {
		if ping.Active {
			activePodName = controller.PodName(int(i))
			break
		}
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(controller.Namespace),
		client.MatchingLabels(labels.NewBuilder().WithControllerSelectorLabels(controller).Build()),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		return err
	}
	sort.Sort(objectutils.PodsByName(podList.Items))

	errs := []error{}
	for i := range podList.Items {
		pod := &podList.Items[i]
		mutateFn := func(pod *corev1.Pod) error {
			if pod.Labels == nil {
				pod.Labels = map[string]string{}
			}
			if pod.Name == activePodName {
				pod.Labels[slinkyv1beta1.LabelControllerActive] = "true"
			} else {
				delete(pod.Labels, slinkyv1beta1.LabelControllerActive)
			}
			return nil
		}
		if err := objectutils.PatchObject(r.Client, ctx, pod, mutateFn); err != nil {
			if !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("failed to patch controller pod (%s): %w", klog.KObj(pod), err))
			}
		}
	}
	if err := utilerrors.NewAggregate(errs); err != nil {
		return err
	}

	return nil
}
