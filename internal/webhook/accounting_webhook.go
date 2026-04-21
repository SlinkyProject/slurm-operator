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

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=slinky.slurm.net,resources=accountings,verbs=delete;create;update

type AccountingWebhook struct{}

// log is for logging in this package.
var accountinglog = logf.Log.WithName("accounting-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *AccountingWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.Accounting{}).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-accounting,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=accountings,verbs=create;update,versions=v1beta1,name=accounting-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ admission.Validator[*slinkyv1beta1.Accounting] = &AccountingWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AccountingWebhook) ValidateCreate(ctx context.Context, accounting *slinkyv1beta1.Accounting) (admission.Warnings, error) {
	accountinglog.Info("validate create", "accounting", klog.KObj(accounting))

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AccountingWebhook) ValidateUpdate(ctx context.Context, oldAccounting, newAccounting *slinkyv1beta1.Accounting) (admission.Warnings, error) {
	accountinglog.Info("validate update", "newAccounting", klog.KObj(newAccounting))

	var warns admission.Warnings
	var errs []error

	if !apiequality.Semantic.DeepEqual(newAccounting.AuthJwtRef(), oldAccounting.AuthJwtRef()) {
		errs = append(errs, errors.New("the value of JwtKeyRef or JwtHs256KeyRef cannot be modified after deployment"))
	}

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AccountingWebhook) ValidateDelete(ctx context.Context, accounting *slinkyv1beta1.Accounting) (admission.Warnings, error) {
	accountinglog.Info("validate delete", "accounting", klog.KObj(accounting))

	return nil, nil
}
