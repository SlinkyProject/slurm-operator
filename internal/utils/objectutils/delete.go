// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"context"
	"errors"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

const (
	ReasonDeleteSucceeded = "DeleteSucceeded"
	ReasonDeleteFailed    = "DeleteFailed"
)

func DeleteObject(c client.Client, ctx context.Context, eventRecorder events.EventRecorder, eventObj client.Object, newObj client.Object) error {
	logger := log.FromContext(ctx)

	var oldObj client.Object
	switch newObj.(type) {
	case *corev1.ConfigMap:
		oldObj = &corev1.ConfigMap{}
	case *corev1.Secret:
		oldObj = &corev1.Secret{}
	case *corev1.Service:
		oldObj = &corev1.Service{}
	case *appsv1.Deployment:
		oldObj = &appsv1.Deployment{}
	case *appsv1.StatefulSet:
		oldObj = &appsv1.StatefulSet{}
	case *slinkyv1beta1.Controller:
		oldObj = &slinkyv1beta1.Controller{}
	case *slinkyv1beta1.RestApi:
		oldObj = &slinkyv1beta1.RestApi{}
	case *slinkyv1beta1.Accounting:
		oldObj = &slinkyv1beta1.Accounting{}
	case *slinkyv1beta1.NodeSet:
		oldObj = &slinkyv1beta1.NodeSet{}
	case *slinkyv1beta1.LoginSet:
		oldObj = &slinkyv1beta1.LoginSet{}
	case *policyv1.PodDisruptionBudget:
		oldObj = &policyv1.PodDisruptionBudget{}
	case *monitoringv1.ServiceMonitor:
		oldObj = &monitoringv1.ServiceMonitor{}
	default:
		return errors.New("unhandled object, this is a bug")
	}

	key := client.ObjectKeyFromObject(newObj)
	if err := c.Get(ctx, key, oldObj); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("error getting %s: %w", key, err)
	}

	// If the object is being deleted, do not update it
	if !oldObj.GetDeletionTimestamp().IsZero() {
		logger.V(1).Info(fmt.Sprintf("%s is being deleted. Skipping...", key))
		return nil
	}

	if err := c.Delete(ctx, oldObj); err != nil {
		if eventRecorder != nil {
			eventRecorder.Eventf(eventObj, oldObj, corev1.EventTypeWarning, ReasonDeleteFailed, "Delete", "Error deleting: %T %s: %v", oldObj, key, err)
		}
		return fmt.Errorf("error deleting %s: %w", key, err)
	}

	if eventRecorder != nil {
		eventRecorder.Eventf(eventObj, oldObj, corev1.EventTypeNormal, ReasonDeleteSucceeded, "Delete", "Deleted %T: %s", oldObj, key)
	}
	return nil
}
