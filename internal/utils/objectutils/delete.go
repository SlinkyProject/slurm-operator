// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func DeleteObject(c client.Client, ctx context.Context, newObj client.Object) error {
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
	default:
		return errors.New("unhandled object, this is a bug")
	}

	key := client.ObjectKeyFromObject(newObj)
	if err := c.Get(ctx, key, oldObj); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting %s: %w", key, err)
		}
		return nil
	}

	// If the object is being deleted, do not update it
	if !oldObj.GetDeletionTimestamp().IsZero() {
		logger.V(1).Info(fmt.Sprintf("%s is being deleted. Skipping...", key))
		return nil
	}

	if err := c.Delete(ctx, oldObj); err != nil {
		return fmt.Errorf("error deleting %s: %w", key, err)
	}

	return nil
}
