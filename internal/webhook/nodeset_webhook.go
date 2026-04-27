// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"errors"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// +kubebuilder:rbac:groups=slinky.slurm.net,resources=nodesets,verbs=delete;create;update

type NodeSetWebhook struct{}

// log is for logging in this package.
var nodesetlog = logf.Log.WithName("nodeset-resource")

func (r *NodeSetWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.NodeSet{}).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-nodeset,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=nodesets,verbs=create;update,versions=v1beta1,name=nodeset-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ admission.Validator[*slinkyv1beta1.NodeSet] = &NodeSetWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *NodeSetWebhook) ValidateCreate(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) (admission.Warnings, error) {
	nodesetlog.Info("validate create", "nodeset", klog.KObj(nodeset))

	warns, errs := r.validateNodeSet(nodeset)

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *NodeSetWebhook) ValidateUpdate(ctx context.Context, oldNodeSet, newNodeSet *slinkyv1beta1.NodeSet) (admission.Warnings, error) {
	nodesetlog.Info("validate update", "newNodeSet", klog.KObj(newNodeSet))

	warns, errs := r.validateNodeSet(newNodeSet)

	if !apiequality.Semantic.DeepEqual(newNodeSet.Spec.ControllerRef, oldNodeSet.Spec.ControllerRef) {
		errs = append(errs, errors.New("cannot change controllerRef after deployment"))
	}
	if !apiequality.Semantic.DeepEqual(newNodeSet.Spec.VolumeClaimTemplates, oldNodeSet.Spec.VolumeClaimTemplates) {
		errs = append(errs, errors.New("cannot change volumeClaimTemplates after deployment"))
	}

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *NodeSetWebhook) ValidateDelete(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) (admission.Warnings, error) {
	nodesetlog.Info("validate delete", "nodeset", klog.KObj(nodeset))

	return nil, nil
}

func (r *NodeSetWebhook) validateNodeSet(nodeset *slinkyv1beta1.NodeSet) (admission.Warnings, []error) {
	var warns admission.Warnings
	var errs []error

	if nodeset.Spec.ControllerRef.Name == "" {
		errs = append(errs, errors.New("controllerRef.name must not be empty"))
	}

	if nodeset.Spec.TaintKubeNodes { //nolint:staticcheck // SA1019
		warns = append(warns, "TaintKubeNodes option is deprecated and will removed in the future.")
	}

	if mu := nodeset.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable; mu != nil {
		switch mu.Type {
		case intstr.Int:
			if mu.IntVal < 1 {
				errs = append(errs, fmt.Errorf("maxUnavailable must be > 0, got %d", mu.IntVal))
			}
		case intstr.String:
			if mu.StrVal == "0%" {
				errs = append(errs, errors.New("maxUnavailable must not be 0%"))
			}
		}
	}

	if nodeset.Spec.Ssh.Enabled && nodeset.Spec.Ssh.SssdConfRef.Name == "" {
		errs = append(errs, errors.New("ssh.sssdConfRef.name must not be empty when ssh is enabled"))
	}

	return warns, errs
}
