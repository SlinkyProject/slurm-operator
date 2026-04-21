// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// +kubebuilder:rbac:groups=slinky.slurm.net,resources=loginsets,verbs=delete;create;update

type LoginSetWebhook struct{}

// log is for logging in this package.
var loginsetlog = logf.Log.WithName("loginset-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *LoginSetWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.LoginSet{}).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-loginset,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=loginsets,verbs=create;update,versions=v1beta1,name=loginset-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ admission.Validator[*slinkyv1beta1.LoginSet] = &LoginSetWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LoginSetWebhook) ValidateCreate(ctx context.Context, loginset *slinkyv1beta1.LoginSet) (admission.Warnings, error) {
	loginsetlog.Info("validate create", "loginset", klog.KObj(loginset))

	warns, errs := r.validateLoginSet(loginset)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LoginSetWebhook) ValidateUpdate(ctx context.Context, oldLoginset, newLoginset *slinkyv1beta1.LoginSet) (admission.Warnings, error) {
	loginsetlog.Info("validate update", "newLoginset", klog.KObj(newLoginset))

	warns, errs := r.validateLoginSet(newLoginset)

	if !apiequality.Semantic.DeepEqual(newLoginset.Spec.ControllerRef, oldLoginset.Spec.ControllerRef) {
		errs = append(errs, errors.New("cannot change controllerRef after deployment"))
	}

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LoginSetWebhook) ValidateDelete(ctx context.Context, loginset *slinkyv1beta1.LoginSet) (admission.Warnings, error) {
	loginsetlog.Info("validate delete", "loginset", klog.KObj(loginset))

	return nil, nil
}

func (r *LoginSetWebhook) validateLoginSet(loginset *slinkyv1beta1.LoginSet) (admission.Warnings, []error) {
	var warns admission.Warnings
	var errs []error

	if loginset.Spec.ControllerRef.Name == "" {
		errs = append(errs, errors.New("controllerRef.name must not be empty"))
	}

	if loginset.Spec.SssdConfRef.Name == "" {
		errs = append(errs, errors.New("sssdConfRef.name must not be empty"))
	}

	switch loginset.Spec.Strategy.Type {
	case "",
		appsv1.RollingUpdateDeploymentStrategyType,
		appsv1.RecreateDeploymentStrategyType,
		slinkyv1beta1.OnDeleteLoginSetStrategyType:
	default:
		errs = append(errs, fmt.Errorf("strategy.type must be one of %q, %q, %q; got %q",
			appsv1.RollingUpdateDeploymentStrategyType,
			appsv1.RecreateDeploymentStrategyType,
			slinkyv1beta1.OnDeleteLoginSetStrategyType,
			loginset.Spec.Strategy.Type))
	}

	if loginset.Spec.Strategy.RollingUpdate != nil &&
		loginset.Spec.Strategy.Type != "" &&
		loginset.Spec.Strategy.Type != appsv1.RollingUpdateDeploymentStrategyType {
		errs = append(errs, fmt.Errorf("strategy.rollingUpdate may only be set when strategy.type is %q; got %q",
			appsv1.RollingUpdateDeploymentStrategyType,
			loginset.Spec.Strategy.Type))
	}

	return warns, errs
}
