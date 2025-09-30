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

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

type LoginSetWebhook struct{}

// log is for logging in this package.
var loginsetlog = logf.Log.WithName("loginset-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *LoginSetWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&slinkyv1beta1.LoginSet{}).
		WithValidator(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
//+kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-loginset,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=loginsets,verbs=create;update,versions=v1beta1,name=loginset-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ webhook.CustomValidator = &LoginSetWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LoginSetWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	loginset := obj.(*slinkyv1beta1.LoginSet)
	loginsetlog.Info("validate create", "loginset", klog.KObj(loginset))

	warns, errs := validateLoginSet(loginset)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LoginSetWebhook) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	newLoginset := newObj.(*slinkyv1beta1.LoginSet)
	_ = oldObj.(*slinkyv1beta1.LoginSet)
	loginsetlog.Info("validate update", "newLoginset", klog.KObj(newLoginset))

	warns, errs := validateLoginSet(newLoginset)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LoginSetWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	loginset := obj.(*slinkyv1beta1.LoginSet)
	loginsetlog.Info("validate delete", "loginset", klog.KObj(loginset))

	return nil, nil
}

func validateLoginSet(obj *slinkyv1beta1.LoginSet) (admission.Warnings, []error) {
	var warns admission.Warnings
	var errs []error

	return warns, errs
}
