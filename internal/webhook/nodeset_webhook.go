// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
)

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

type NodeSetWebhook struct{}

// log is for logging in this package.
var nodesetlog = logf.Log.WithName("nodeset-resource")

func (r *NodeSetWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&slinkyv1alpha1.NodeSet{}).
		WithValidator(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-slinky-slurm-net-v1alpha1-nodeset,mutating=false,failurePolicy=fail,sideEffects=None,groups=slinky.slurm.net,resources=nodesets,verbs=create;update,versions=v1alpha1,name=nodeset-v1alpha1.kb.io,admissionReviewVersions=v1;v1alpha1

var _ webhook.CustomValidator = &NodeSetWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *NodeSetWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	nodeset := obj.(*slinkyv1alpha1.NodeSet)
	nodesetlog.Info("validate create", "nodeset", klog.KObj(nodeset))

	warns, errs := validateNodeSet(nodeset)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *NodeSetWebhook) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	newNodeSet := newObj.(*slinkyv1alpha1.NodeSet)
	_ = oldObj.(*slinkyv1alpha1.NodeSet)
	nodesetlog.Info("validate update", "newNodeSet", klog.KObj(newNodeSet))

	warns, errs := validateNodeSet(newNodeSet)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *NodeSetWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	nodeset := obj.(*slinkyv1alpha1.NodeSet)
	nodesetlog.Info("validate delete", "nodeset", klog.KObj(nodeset))

	return nil, nil
}

func validateNodeSet(obj *slinkyv1alpha1.NodeSet) (admission.Warnings, []error) {
	var warns admission.Warnings
	var errs []error

	switch obj.Spec.UpdateStrategy.Type {
	case slinkyv1alpha1.RollingUpdateNodeSetStrategyType:
		// valid
	case slinkyv1alpha1.OnDeleteNodeSetStrategyType:
		// valid
	default:
		errs = append(errs, fmt.Errorf("`NodeSet.Spec.UpdateStrategy.Type` is not valid. Got: %v. Expected of: %s; %s",
			obj.Spec.UpdateStrategy.Type, slinkyv1alpha1.RollingUpdateNodeSetStrategyType, slinkyv1alpha1.OnDeleteNodeSetStrategyType))
	}

	if obj.Spec.PersistentVolumeClaimRetentionPolicy != nil {
		switch obj.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted {
		case slinkyv1alpha1.RetainPersistentVolumeClaimRetentionPolicyType:
			// valid
		case slinkyv1alpha1.DeletePersistentVolumeClaimRetentionPolicyType:
			// valid
		default:
			errs = append(errs, fmt.Errorf("`NodeSet.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted` is not valid. Got: %v. Expected of: %s; %s",
				obj.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted, slinkyv1alpha1.RetainPersistentVolumeClaimRetentionPolicyType, slinkyv1alpha1.DeletePersistentVolumeClaimRetentionPolicyType))
		}
		switch obj.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled {
		case slinkyv1alpha1.RetainPersistentVolumeClaimRetentionPolicyType:
			// valid
		case slinkyv1alpha1.DeletePersistentVolumeClaimRetentionPolicyType:
			// valid
		default:
			errs = append(errs, fmt.Errorf("`NodeSet.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled` is not valid. Got: %v. Expected of: %s; %s",
				obj.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled, slinkyv1alpha1.RetainPersistentVolumeClaimRetentionPolicyType, slinkyv1alpha1.DeletePersistentVolumeClaimRetentionPolicyType))
		}
	}

	return warns, errs
}
