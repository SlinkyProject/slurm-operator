// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

type RestapiWebhook struct{}

// log is for logging in this package.
var restapilog = logf.Log.WithName("restapi-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *RestapiWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.RestApi{}).
		WithValidator(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-restapi,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=restapis,verbs=create;update,versions=v1beta1,name=restapi-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ admission.Validator[*slinkyv1beta1.RestApi] = &RestapiWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *RestapiWebhook) ValidateCreate(ctx context.Context, restapi *slinkyv1beta1.RestApi) (admission.Warnings, error) {
	restapilog.Info("validate create", "restapi", klog.KObj(restapi))

	return nil, validateRestApiPodDisruptionBudget(restapi)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *RestapiWebhook) ValidateUpdate(ctx context.Context, oldRestapi, newRestapi *slinkyv1beta1.RestApi) (admission.Warnings, error) {
	restapilog.Info("validate update", "newRestapi", klog.KObj(newRestapi))

	return nil, validateRestApiPodDisruptionBudget(newRestapi)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *RestapiWebhook) ValidateDelete(ctx context.Context, restapi *slinkyv1beta1.RestApi) (admission.Warnings, error) {
	restapilog.Info("validate delete", "restapi", klog.KObj(restapi))

	return nil, nil
}

// validateRestApiPodDisruptionBudget enforces PDB shape when the operator would honor it.
// When replicas is at most one and podDisruptionBudget is enabled (default), validation is skipped
// because the controller does not reconcile a PDB in that case.
func validateRestApiPodDisruptionBudget(restapi *slinkyv1beta1.RestApi) error {
	pdb := restapi.Spec.PodDisruptionBudget
	if pdb == nil {
		return nil
	}
	replicas := ptr.Deref(restapi.Spec.Replicas, 1)
	if replicas <= 1 && ptr.Deref(pdb.Enabled, true) {
		return nil
	}
	var errs []error
	spec := pdb.PodDisruptionBudgetSpec
	if spec.MinAvailable != nil && spec.MaxUnavailable != nil {
		errs = append(errs, fmt.Errorf("spec.podDisruptionBudget: minAvailable and maxUnavailable cannot both be set"))
	}
	return utilerrors.NewAggregate(errs)
}
