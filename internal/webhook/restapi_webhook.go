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

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
)

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

type RestapiWebhook struct{}

// log is for logging in this package.
var restapilog = logf.Log.WithName("restapi-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *RestapiWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&slinkyv1alpha1.RestApi{}).
		WithValidator(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1alpha1-restapi,mutating=false,failurePolicy=fail,sideEffects=None,groups=slinky.slurm.net,resources=restapis,verbs=create;update,versions=v1alpha1,name=vrestapi.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &RestapiWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *RestapiWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	restapi := obj.(*slinkyv1alpha1.RestApi)
	restapilog.Info("validate create", "restapi", klog.KObj(restapi))

	warns, errs := validateRestapi(restapi)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *RestapiWebhook) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	newRestapi := newObj.(*slinkyv1alpha1.RestApi)
	_ = oldObj.(*slinkyv1alpha1.RestApi)
	restapilog.Info("validate update", "newRestapi", klog.KObj(newRestapi))

	warns, errs := validateRestapi(newRestapi)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *RestapiWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	restapi := obj.(*slinkyv1alpha1.RestApi)
	restapilog.Info("validate delete", "restapi", klog.KObj(restapi))

	return nil, nil
}

func validateRestapi(obj *slinkyv1alpha1.RestApi) (admission.Warnings, []error) {
	var warns admission.Warnings
	var errs []error

	return warns, errs
}
