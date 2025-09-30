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

type TokenWebhook struct{}

// log is for logging in this package.
var tokenlog = logf.Log.WithName("token-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *TokenWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&slinkyv1beta1.Token{}).
		WithValidator(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-token,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=tokens,verbs=create;update,versions=v1beta1,name=token-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ webhook.CustomValidator = &TokenWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *TokenWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	token := obj.(*slinkyv1beta1.Token)
	tokenlog.Info("validate create", "token", klog.KObj(token))

	warns, errs := validateToken(token)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *TokenWebhook) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	newToken := newObj.(*slinkyv1beta1.Token)
	_ = oldObj.(*slinkyv1beta1.Token)
	tokenlog.Info("validate update", "newToken", klog.KObj(newToken))

	warns, errs := validateToken(newToken)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *TokenWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	token := obj.(*slinkyv1beta1.Token)
	tokenlog.Info("validate delete", "token", klog.KObj(token))

	return nil, nil
}

func validateToken(obj *slinkyv1beta1.Token) (admission.Warnings, []error) {
	var warns admission.Warnings
	var errs []error

	return warns, errs
}
