// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
)

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

type AccountingSetWebhook struct{}

// log is for logging in this package.
var accountinglog = logf.Log.WithName("accounting-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *AccountingSetWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&slinkyv1beta1.Accounting{}).
		WithValidator(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-accounting,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=accountings,verbs=create;update,versions=v1beta1,name=accounting-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ webhook.CustomValidator = &AccountingSetWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AccountingSetWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	accounting := obj.(*slinkyv1beta1.Accounting)
	accountinglog.Info("validate create", "accounting", klog.KObj(accounting))

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AccountingSetWebhook) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	newAccounting := newObj.(*slinkyv1beta1.Accounting)
	_ = oldObj.(*slinkyv1beta1.Accounting)
	accountinglog.Info("validate update", "newAccounting", klog.KObj(newAccounting))

	warns, errs := validateAccounting(newAccounting)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AccountingSetWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	accounting := obj.(*slinkyv1beta1.Accounting)
	accountinglog.Info("validate delete", "accounting", klog.KObj(accounting))

	return nil, nil
}

func validateAccounting(obj *slinkyv1beta1.Accounting) (admission.Warnings, []error) {
	var warns admission.Warnings
	var errs []error

	return warns, errs
}
