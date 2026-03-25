// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

type SlurmQosWebhook struct {
	client.Client
}

// log is for logging in this package.
var slurmqoslog = logf.Log.WithName("slurmqos-resource")

func (r *SlurmQosWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.SlurmQos{}).
		WithValidator(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-slurmqos,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=slurmqos,verbs=create;update,versions=v1beta1,name=slurmqos-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ admission.Validator[*slinkyv1beta1.SlurmQos] = &SlurmQosWebhook{}

var qosNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *SlurmQosWebhook) ValidateCreate(ctx context.Context, qos *slinkyv1beta1.SlurmQos) (admission.Warnings, error) {
	slurmqoslog.Info("validate create", "slurmqos", klog.KObj(qos))

	warns, errs := r.validateSlurmQos(ctx, qos)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *SlurmQosWebhook) ValidateUpdate(ctx context.Context, oldQos, newQos *slinkyv1beta1.SlurmQos) (admission.Warnings, error) {
	slurmqoslog.Info("validate update", "newSlurmQos", klog.KObj(newQos))

	warns, errs := r.validateSlurmQos(ctx, newQos)

	if !apiequality.Semantic.DeepEqual(newQos.Spec.ControllerRef, oldQos.Spec.ControllerRef) {
		errs = append(errs, errors.New("cannot change controllerRef after deployment"))
	}

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *SlurmQosWebhook) ValidateDelete(ctx context.Context, qos *slinkyv1beta1.SlurmQos) (admission.Warnings, error) {
	slurmqoslog.Info("validate delete", "slurmqos", klog.KObj(qos))

	return nil, nil
}

func (r *SlurmQosWebhook) validateSlurmQos(ctx context.Context, qos *slinkyv1beta1.SlurmQos) (admission.Warnings, []error) {
	var warns admission.Warnings
	var errs []error

	if qos.Spec.ControllerRef.Name == "" {
		errs = append(errs, errors.New("controllerRef.name must not be empty"))
	}

	if !qosNameRegexp.MatchString(qos.Name) {
		errs = append(errs, fmt.Errorf("metadata.name %q must contain only alphanumeric characters, hyphens, or underscores", qos.Name))
	}
	if len(qos.Name) > 255 {
		errs = append(errs, fmt.Errorf("metadata.name %q exceeds 255 characters (%d)", qos.Name, len(qos.Name)))
	}

	if qos.Spec.Config != nil && qos.Spec.Config.Raw != nil {
		var raw map[string]interface{}
		if err := json.Unmarshal(qos.Spec.Config.Raw, &raw); err != nil {
			errs = append(errs, fmt.Errorf("config must be valid JSON: %w", err))
		}
	}

	// Verify referenced Controller has accounting enabled.
	// Client may be nil in unit tests that validate field-level rules only.
	if qos.Spec.ControllerRef.Name != "" && r.Client != nil {
		controller := &slinkyv1beta1.Controller{}
		controllerKey := qos.Spec.ControllerRef.NamespacedName()
		if controllerKey.Namespace == "" {
			controllerKey.Namespace = qos.Namespace
		}
		if err := r.Get(ctx, controllerKey, controller); err != nil {
			errs = append(errs, fmt.Errorf("failed to get referenced Controller %q: %w", controllerKey, err))
		} else if controller.Spec.AccountingRef == nil {
			errs = append(errs, fmt.Errorf("referenced Controller %q does not have accounting enabled (accountingRef is nil); QOS requires accounting", controllerKey))
		}
	}

	if qos.Name == "normal" {
		warns = append(warns, "Slurm automatically creates a 'normal' QOS. This SlurmQos will update the existing 'normal' QOS.")
	}

	return warns, errs
}
