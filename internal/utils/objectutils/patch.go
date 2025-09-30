// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func SyncObject(c client.Client, ctx context.Context, newObj client.Object, shouldUpdate bool) error {
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
	default:
		return errors.New("unhandled object, this is a bug")
	}

	key := client.ObjectKeyFromObject(newObj)
	if err := c.Get(ctx, key, oldObj); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting %s: %w", key, err)
		}
		if err := c.Create(ctx, newObj); err != nil {
			return fmt.Errorf("error creating %s: %w", key, err)
		}
		return nil
	}

	// If the object is being deleted, do not update it
	if !oldObj.GetDeletionTimestamp().IsZero() {
		logger.V(1).Info(fmt.Sprintf("%s is being deleted. Skipping...", key))
		return nil
	}

	if !shouldUpdate {
		return nil
	}

	var patch client.Patch
	switch o := newObj.(type) {
	case *corev1.ConfigMap:
		obj := oldObj.(*corev1.ConfigMap)
		if ptr.Deref(obj.Immutable, false) {
			logger.V(1).Info(fmt.Sprintf("%s is immutable. Skipping...", key))
			return nil
		}
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Data = o.Data
		obj.BinaryData = o.BinaryData
	case *corev1.Secret:
		obj := oldObj.(*corev1.Secret)
		if ptr.Deref(obj.Immutable, false) {
			logger.V(1).Info(fmt.Sprintf("%s is immutable. Skipping...", key))
			return nil
		}
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Data = o.Data
		obj.StringData = o.StringData
	case *corev1.Service:
		obj := oldObj.(*corev1.Service)
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Spec = o.Spec
	case *appsv1.Deployment:
		obj := oldObj.(*appsv1.Deployment)
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Spec.MinReadySeconds = o.Spec.MinReadySeconds
		obj.Spec.Replicas = o.Spec.Replicas
		obj.Spec.Strategy = o.Spec.Strategy
		obj.Spec.Template = o.Spec.Template
	case *appsv1.StatefulSet:
		obj := oldObj.(*appsv1.StatefulSet)
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Spec.MinReadySeconds = o.Spec.MinReadySeconds
		obj.Spec.Ordinals = o.Spec.Ordinals
		obj.Spec.PersistentVolumeClaimRetentionPolicy = o.Spec.PersistentVolumeClaimRetentionPolicy
		obj.Spec.Replicas = o.Spec.Replicas
		obj.Spec.Template = o.Spec.Template
		obj.Spec.UpdateStrategy = o.Spec.UpdateStrategy
	case *slinkyv1beta1.Controller:
		obj := oldObj.(*slinkyv1beta1.Controller)
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Spec = o.Spec
	case *slinkyv1beta1.RestApi:
		obj := oldObj.(*slinkyv1beta1.RestApi)
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Spec = o.Spec
	case *slinkyv1beta1.Accounting:
		obj := oldObj.(*slinkyv1beta1.Accounting)
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Spec = o.Spec
	case *slinkyv1beta1.NodeSet:
		obj := oldObj.(*slinkyv1beta1.NodeSet)
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Spec.MinReadySeconds = o.Spec.MinReadySeconds
		obj.Spec.PersistentVolumeClaimRetentionPolicy = o.Spec.PersistentVolumeClaimRetentionPolicy
		obj.Spec.Replicas = o.Spec.Replicas
		obj.Spec.Template = o.Spec.Template
		obj.Spec.UpdateStrategy = o.Spec.UpdateStrategy
	case *slinkyv1beta1.LoginSet:
		obj := oldObj.(*slinkyv1beta1.LoginSet)
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Spec.Replicas = o.Spec.Replicas
		obj.Spec.Template = o.Spec.Template
	case *policyv1.PodDisruptionBudget:
		obj := oldObj.(*policyv1.PodDisruptionBudget)
		patch = client.MergeFrom(obj.DeepCopy())
		obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
		obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
		obj.Spec.MaxUnavailable = o.Spec.MaxUnavailable
		obj.Spec.MinAvailable = o.Spec.MinAvailable
		obj.Spec.Selector = o.Spec.Selector
	default:
		return errors.New("unhandled patch object, this is a bug")
	}

	if err := c.Patch(ctx, oldObj, patch); err != nil {
		return fmt.Errorf("error patching %s: %w", key, err)
	}

	return nil
}
