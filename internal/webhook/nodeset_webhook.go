// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

type NodeSetWebhook struct{}

// log is for logging in this package.
var nodesetlog = logf.Log.WithName("nodeset-resource")

func (r *NodeSetWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.NodeSet{}).
		WithValidator(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-nodeset,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=nodesets,verbs=create;update,versions=v1beta1,name=nodeset-v1beta1.kb.io,admissionReviewVersions=v1beta1

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

	hostname := nodeset.Spec.Template.PodSpecWrapper.Hostname
	if hostname != "" {
		for _, msg := range apivalidation.NameIsDNSSubdomain(hostname, true) {
			errs = append(errs, fmt.Errorf("template.spec.hostname: %s", msg))
		}
	}

	return warns, errs
}
