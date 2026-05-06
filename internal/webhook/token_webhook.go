// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"errors"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// +kubebuilder:rbac:groups=slinky.slurm.net,resources=tokens,verbs=delete;create;update

type TokenWebhook struct{}

// log is for logging in this package.
var tokenlog = logf.Log.WithName("token-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *TokenWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.Token{}).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-token,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=tokens,verbs=create;update,versions=v1beta1,name=token-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ admission.Validator[*slinkyv1beta1.Token] = &TokenWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *TokenWebhook) ValidateCreate(ctx context.Context, token *slinkyv1beta1.Token) (admission.Warnings, error) {
	tokenlog.Info("validate create", "token", klog.KObj(token))

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *TokenWebhook) ValidateUpdate(ctx context.Context, oldToken, newToken *slinkyv1beta1.Token) (admission.Warnings, error) {
	tokenlog.Info("validate update", "newToken", klog.KObj(newToken))

	var warns admission.Warnings
	var errs []error

	if !apiequality.Semantic.DeepEqual(newToken.JwtRef(), oldToken.JwtRef()) {
		errs = append(errs, errors.New("the value of JwtKeyRef or JwtHs256KeyRef cannot be modified after deployment"))
	}

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *TokenWebhook) ValidateDelete(ctx context.Context, token *slinkyv1beta1.Token) (admission.Warnings, error) {
	tokenlog.Info("validate delete", "token", klog.KObj(token))

	return nil, nil
}
