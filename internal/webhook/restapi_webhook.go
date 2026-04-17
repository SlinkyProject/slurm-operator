// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// +kubebuilder:rbac:groups=slinky.slurm.net,resources=restapis,verbs=delete;create;update

type RestapiWebhook struct{}

// log is for logging in this package.
var restapilog = logf.Log.WithName("restapi-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *RestapiWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.RestApi{}).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-restapi,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=restapis,verbs=create;update,versions=v1beta1,name=restapi-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ admission.Validator[*slinkyv1beta1.RestApi] = &RestapiWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *RestapiWebhook) ValidateCreate(ctx context.Context, restapi *slinkyv1beta1.RestApi) (admission.Warnings, error) {
	restapilog.Info("validate create", "restapi", klog.KObj(restapi))

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *RestapiWebhook) ValidateUpdate(ctx context.Context, oldRestapi, newRestapi *slinkyv1beta1.RestApi) (admission.Warnings, error) {
	restapilog.Info("validate update", "newRestapi", klog.KObj(newRestapi))

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *RestapiWebhook) ValidateDelete(ctx context.Context, restapi *slinkyv1beta1.RestApi) (admission.Warnings, error) {
	restapilog.Info("validate delete", "restapi", klog.KObj(restapi))

	return nil, nil
}
