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
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// +kubebuilder:rbac:groups=slinky.slurm.net,resources=accounts,verbs=delete;create;update

type AccountWebhook struct{}

var accountlog = logf.Log.WithName("account-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *AccountWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.Account{}).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-account,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=accounts,verbs=create;update,versions=v1beta1,name=account-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ admission.Validator[*slinkyv1beta1.Account] = &AccountWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AccountWebhook) ValidateCreate(ctx context.Context, account *slinkyv1beta1.Account) (admission.Warnings, error) {
	accountlog.Info("validate create", "account", klog.KObj(account))
	return nil, validateAccount(account)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AccountWebhook) ValidateUpdate(ctx context.Context, oldAccount, newAccount *slinkyv1beta1.Account) (admission.Warnings, error) {
	accountlog.Info("validate update", "account", klog.KObj(newAccount))

	var errs []error
	if !apiequality.Semantic.DeepEqual(newAccount.Spec.ControllerRef, oldAccount.Spec.ControllerRef) {
		errs = append(errs, errors.New("spec.controllerRef is immutable"))
	}
	if err := validateAccount(newAccount); err != nil {
		errs = append(errs, err)
	}
	return nil, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AccountWebhook) ValidateDelete(ctx context.Context, account *slinkyv1beta1.Account) (admission.Warnings, error) {
	accountlog.Info("validate delete", "account", klog.KObj(account))
	return nil, nil
}

func validateAccount(account *slinkyv1beta1.Account) error {
	var errs []error
	if account.Spec.ControllerRef.Name == "" {
		errs = append(errs, errors.New("spec.controllerRef.name must be set"))
	}
	errs = append(errs, validateAssociationLimits("spec.limits", account.Spec.Limits)...)
	return utilerrors.NewAggregate(errs)
}

// validateAssociationLimits validates the shared association limits block.
func validateAssociationLimits(path string, l slinkyv1beta1.AssociationLimits) []error {
	var errs []error
	if l.DefaultQOS != nil && len(l.QOS) > 0 {
		found := false
		for _, q := range l.QOS {
			if q == *l.DefaultQOS {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, fmt.Errorf("%s.defaultQos %q must be present in %s.qos", path, *l.DefaultQOS, path))
		}
	}
	return errs
}
