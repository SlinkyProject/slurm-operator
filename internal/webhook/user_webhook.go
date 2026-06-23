// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"errors"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// +kubebuilder:rbac:groups=slinky.slurm.net,resources=users,verbs=delete;create;update

type UserWebhook struct{}

var userlog = logf.Log.WithName("user-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *UserWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.User{}).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-user,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=users,verbs=create;update,versions=v1beta1,name=user-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ admission.Validator[*slinkyv1beta1.User] = &UserWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *UserWebhook) ValidateCreate(ctx context.Context, user *slinkyv1beta1.User) (admission.Warnings, error) {
	userlog.Info("validate create", "user", klog.KObj(user))
	return nil, validateUser(user)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *UserWebhook) ValidateUpdate(ctx context.Context, oldUser, newUser *slinkyv1beta1.User) (admission.Warnings, error) {
	userlog.Info("validate update", "user", klog.KObj(newUser))

	var errs []error
	if !apiequality.Semantic.DeepEqual(newUser.Spec.ControllerRef, oldUser.Spec.ControllerRef) {
		errs = append(errs, errors.New("spec.controllerRef is immutable"))
	}
	if err := validateUser(newUser); err != nil {
		errs = append(errs, err)
	}
	return nil, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *UserWebhook) ValidateDelete(ctx context.Context, user *slinkyv1beta1.User) (admission.Warnings, error) {
	userlog.Info("validate delete", "user", klog.KObj(user))
	return nil, nil
}

func validateUser(user *slinkyv1beta1.User) error {
	var errs []error
	if user.Spec.ControllerRef.Name == "" {
		errs = append(errs, errors.New("spec.controllerRef.name must be set"))
	}
	if len(user.Spec.Associations) == 0 {
		errs = append(errs, errors.New("spec.associations must have at least one entry"))
	}

	seen := make(map[string]struct{}, len(user.Spec.Associations))
	defaultAccountFound := user.Spec.DefaultAccount == ""
	for i, ua := range user.Spec.Associations {
		path := fmt.Sprintf("spec.associations[%d]", i)
		if ua.Account == "" {
			errs = append(errs, fmt.Errorf("%s.account must be set", path))
		}
		key := ua.Account + "/" + ptr.Deref(ua.Partition, "")
		if _, dup := seen[key]; dup {
			errs = append(errs, fmt.Errorf("%s duplicates account/partition %q", path, key))
		}
		seen[key] = struct{}{}
		if ua.Account == user.Spec.DefaultAccount {
			defaultAccountFound = true
		}
		errs = append(errs, validateAssociationLimits(path+".limits", ua.Limits)...)
	}
	if !defaultAccountFound {
		errs = append(errs, fmt.Errorf("spec.defaultAccount %q must match one of spec.associations[].account", user.Spec.DefaultAccount))
	}
	return utilerrors.NewAggregate(errs)
}
