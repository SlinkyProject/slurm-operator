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
	"k8s.io/apimachinery/pkg/api/equality"
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
	case *monitoringv1.ServiceMonitor:
		oldObj = &monitoringv1.ServiceMonitor{}
	default:
		return errors.New("unhandled object, this is a bug")
	}

	key := client.ObjectKeyFromObject(newObj)
	if err := c.Get(ctx, key, oldObj); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting %s: %w", key, err)
		}
		if err := c.Create(ctx, newObj); err != nil {
			if apierrors.IsAlreadyExists(err) {
				if err := c.Get(ctx, key, oldObj); err != nil {
					return fmt.Errorf("error getting %s: %w", key, err)
				}
			} else {
				return fmt.Errorf("error creating %s: %w", key, err)
			}
		} else {
			return nil
		}
	}

	// If the object is being deleted, do not update it
	if !oldObj.GetDeletionTimestamp().IsZero() {
		logger.V(1).Info(fmt.Sprintf("%s is being deleted. Skipping...", key))
		return nil
	}

	if !shouldUpdate {
		return nil
	}

	var patchErr error
	switch o := newObj.(type) {
	case *corev1.ConfigMap:
		obj := oldObj.(*corev1.ConfigMap)
		if ptr.Deref(obj.Immutable, false) {
			logger.V(1).Info(fmt.Sprintf("%s is immutable. Skipping...", key))
			return nil
		}
		patchErr = PatchObject(c, ctx, obj, func(obj *corev1.ConfigMap) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Data = o.Data
			obj.BinaryData = o.BinaryData
			return nil
		})
	case *corev1.Secret:
		obj := oldObj.(*corev1.Secret)
		if ptr.Deref(obj.Immutable, false) {
			logger.V(1).Info(fmt.Sprintf("%s is immutable. Skipping...", key))
			return nil
		}
		patchErr = PatchObject(c, ctx, obj, func(obj *corev1.Secret) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Data = o.Data
			obj.StringData = o.StringData
			return nil
		})
	case *corev1.Service:
		obj := oldObj.(*corev1.Service)
		patchErr = PatchObject(c, ctx, obj, func(obj *corev1.Service) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Spec = o.Spec
			return nil
		})
	case *appsv1.Deployment:
		obj := oldObj.(*appsv1.Deployment)
		patchErr = PatchObject(c, ctx, obj, func(obj *appsv1.Deployment) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Spec.MinReadySeconds = o.Spec.MinReadySeconds
			obj.Spec.Replicas = o.Spec.Replicas
			obj.Spec.Strategy = o.Spec.Strategy
			obj.Spec.Template = o.Spec.Template
			return nil
		})
	case *appsv1.StatefulSet:
		obj := oldObj.(*appsv1.StatefulSet)
		patchErr = PatchObject(c, ctx, obj, func(obj *appsv1.StatefulSet) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Spec.MinReadySeconds = o.Spec.MinReadySeconds
			obj.Spec.Ordinals = o.Spec.Ordinals
			obj.Spec.PersistentVolumeClaimRetentionPolicy = o.Spec.PersistentVolumeClaimRetentionPolicy
			obj.Spec.Replicas = o.Spec.Replicas
			obj.Spec.Template = o.Spec.Template
			obj.Spec.UpdateStrategy = o.Spec.UpdateStrategy
			return nil
		})
	case *slinkyv1beta1.Controller:
		obj := oldObj.(*slinkyv1beta1.Controller)
		patchErr = PatchObject(c, ctx, obj, func(obj *slinkyv1beta1.Controller) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Spec = o.Spec
			return nil
		})
	case *slinkyv1beta1.RestApi:
		obj := oldObj.(*slinkyv1beta1.RestApi)
		patchErr = PatchObject(c, ctx, obj, func(obj *slinkyv1beta1.RestApi) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Spec = o.Spec
			return nil
		})
	case *slinkyv1beta1.Accounting:
		obj := oldObj.(*slinkyv1beta1.Accounting)
		patchErr = PatchObject(c, ctx, obj, func(obj *slinkyv1beta1.Accounting) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Spec = o.Spec
			return nil
		})
	case *slinkyv1beta1.NodeSet:
		obj := oldObj.(*slinkyv1beta1.NodeSet)
		patchErr = PatchObject(c, ctx, obj, func(obj *slinkyv1beta1.NodeSet) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Spec.MinReadySeconds = o.Spec.MinReadySeconds
			obj.Spec.PersistentVolumeClaimRetentionPolicy = o.Spec.PersistentVolumeClaimRetentionPolicy
			obj.Spec.Replicas = o.Spec.Replicas
			obj.Spec.Template = o.Spec.Template
			obj.Spec.UpdateStrategy = o.Spec.UpdateStrategy
			return nil
		})
	case *slinkyv1beta1.LoginSet:
		obj := oldObj.(*slinkyv1beta1.LoginSet)
		patchErr = PatchObject(c, ctx, obj, func(obj *slinkyv1beta1.LoginSet) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Spec.Replicas = o.Spec.Replicas
			obj.Spec.Template = o.Spec.Template
			return nil
		})
	case *policyv1.PodDisruptionBudget:
		obj := oldObj.(*policyv1.PodDisruptionBudget)
		patchErr = PatchObject(c, ctx, obj, func(obj *policyv1.PodDisruptionBudget) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Spec.MaxUnavailable = o.Spec.MaxUnavailable
			obj.Spec.MinAvailable = o.Spec.MinAvailable
			obj.Spec.Selector = o.Spec.Selector
			return nil
		})
	case *monitoringv1.ServiceMonitor:
		obj := oldObj.(*monitoringv1.ServiceMonitor)
		patchErr = PatchObject(c, ctx, obj, func(obj *monitoringv1.ServiceMonitor) error {
			obj.Annotations = structutils.MergeMaps(obj.Annotations, o.Annotations)
			obj.Labels = structutils.MergeMaps(obj.Labels, o.Labels)
			if !equality.Semantic.DeepEqual(obj.OwnerReferences, o.OwnerReferences) {
				obj.OwnerReferences = o.OwnerReferences
			}
			obj.Spec.JobLabel = o.Spec.JobLabel
			obj.Spec.TargetLabels = o.Spec.TargetLabels
			obj.Spec.PodTargetLabels = o.Spec.PodTargetLabels
			obj.Spec.Endpoints = o.Spec.Endpoints
			obj.Spec.Selector = o.Spec.Selector
			obj.Spec.SelectorMechanism = o.Spec.SelectorMechanism
			obj.Spec.NamespaceSelector = o.Spec.NamespaceSelector
			obj.Spec.SampleLimit = o.Spec.SampleLimit
			obj.Spec.ScrapeProtocols = o.Spec.ScrapeProtocols
			obj.Spec.FallbackScrapeProtocol = o.Spec.FallbackScrapeProtocol
			obj.Spec.TargetLimit = o.Spec.TargetLimit
			obj.Spec.LabelLimit = o.Spec.LabelLimit
			obj.Spec.LabelNameLengthLimit = o.Spec.LabelNameLengthLimit
			obj.Spec.LabelValueLengthLimit = o.Spec.LabelValueLengthLimit
			obj.Spec.NativeHistogramConfig = o.Spec.NativeHistogramConfig
			obj.Spec.KeepDroppedTargets = o.Spec.KeepDroppedTargets
			obj.Spec.AttachMetadata = o.Spec.AttachMetadata
			obj.Spec.ScrapeClassName = o.Spec.ScrapeClassName
			obj.Spec.BodySizeLimit = o.Spec.BodySizeLimit
			obj.Spec.ServiceDiscoveryRole = o.Spec.ServiceDiscoveryRole
			return nil
		})
	default:
		return errors.New("unhandled patch object, this is a bug")
	}
	if patchErr != nil {
		return fmt.Errorf("error patching %s: %w", key, patchErr)
	}
	return nil
}

func PatchObject[T client.Object](c client.Client, ctx context.Context, obj T, mutateFn func(T) error) error {
	key := client.ObjectKeyFromObject(obj)
	patch, err := createPatch(c, ctx, obj, mutateFn)
	if err != nil {
		return fmt.Errorf("failed to create object patch for %s: %w", key, err)
	}
	data, err := patch.Data(obj)
	if err != nil {
		return err
	} else if string(data) == "{}" {
		return nil
	}
	if err := c.Patch(ctx, obj, patch); err != nil {
		return fmt.Errorf("failed to patch %s: %w", key, err)
	}
	return nil
}

func StatusPatchObject[T client.Object](c client.Client, ctx context.Context, obj T, mutateFn func(T) error) error {
	key := client.ObjectKeyFromObject(obj)
	patch, err := createPatch(c, ctx, obj, mutateFn)
	if err != nil {
		return fmt.Errorf("failed to create object patch for %s: %w", key, err)
	}
	data, err := patch.Data(obj)
	if err != nil {
		return err
	} else if string(data) == "{}" {
		return nil
	}
	if err := c.Status().Patch(ctx, obj, patch); err != nil {
		return fmt.Errorf("failed to patch %s: %w", key, err)
	}
	return nil
}

func createPatch[T client.Object](c client.Client, ctx context.Context, obj T, mutateFn func(T) error) (client.Patch, error) {
	if mutateFn == nil {
		return nil, errors.New("mutateFn is required")
	}
	key := client.ObjectKeyFromObject(obj)
	if err := c.Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("error getting %s: %w", key, err)
	}
	baseline, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return nil, fmt.Errorf("DeepCopy of %T did not yield a client.Object", obj)
	}
	if err := mutateFn(obj); err != nil {
		return nil, err
	}
	patch := client.MergeFrom(baseline)
	return patch, nil
}
