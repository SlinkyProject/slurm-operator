// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

type ControllerWebhook struct {
	client.Client
}

// log is for logging in this package.
var controllerlog = logf.Log.WithName("controller-resource")

func (r *ControllerWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&slinkyv1beta1.Controller{}).
		WithValidator(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-controller,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=controllers,verbs=create;update,versions=v1beta1,name=controller-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ webhook.CustomValidator = &ControllerWebhook{}

const validTableNameRegex = `[0-9a-zA-Z$_]+`
const warnTableNameRegex = `[A-Z]+`

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ControllerWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	controller := obj.(*slinkyv1beta1.Controller)
	controllerlog.Info("validate create", "controller", klog.KObj(controller))

	warns, errs := r.validateController(ctx, controller)

	// https://slurm.schedmd.com/slurm.conf.html#OPT_ClusterName
	controllerName := controller.ClusterName()
	if len(controllerName) > 40 {
		errs = append(errs, fmt.Errorf("ClusterName exceeds 40 characters (%d): %s",
			len(controllerName), controllerName))
	}
	validTableName := regexp.MustCompile(validTableNameRegex)
	if !validTableName.MatchString(controllerName) {
		errs = append(errs, fmt.Errorf("ClusterName must match regex `%s`: %s",
			validTableNameRegex, controllerName))
	}
	warnTableName := regexp.MustCompile(warnTableNameRegex)
	if warnTableName.MatchString(controllerName) {
		warns = append(warns, fmt.Sprintf("ClusterName contains capital letters: %s",
			controllerName))
	}

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ControllerWebhook) ValidateUpdate(ctx context.Context, oldObj runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	newController := newObj.(*slinkyv1beta1.Controller)
	oldController := oldObj.(*slinkyv1beta1.Controller)
	controllerlog.Info("validate update", "newController", klog.KObj(newController))

	warns, errs := r.validateController(ctx, newController)

	if newController.ClusterName() != oldController.ClusterName() {
		errs = append(errs, errors.New("cannot change ClusterName after deployment"))
	}
	if !apiequality.Semantic.DeepEqual(newController.Spec.SlurmKeyRef.LocalObjectReference, oldController.Spec.SlurmKeyRef.LocalObjectReference) {
		errs = append(errs, errors.New("cannot change SlurmKeyRef after deployment"))
	}
	if !apiequality.Semantic.DeepEqual(newController.Spec.JwtHs256KeyRef.LocalObjectReference, oldController.Spec.JwtHs256KeyRef.LocalObjectReference) {
		errs = append(errs, errors.New("cannot change JwtHs256KeyRef after deployment"))
	}

	// We use volumeClaimTemplates to handle the controller savestate PVC.
	// StatefulSet does not allow update of that field.
	if newController.Spec.Persistence.Enabled != oldController.Spec.Persistence.Enabled {
		errs = append(errs, errors.New("cannot change persistence.enabled after deployment"))
	}

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ControllerWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	controller := obj.(*slinkyv1beta1.Controller)
	controllerlog.Info("validate delete", "controller", klog.KObj(controller))

	return nil, nil
}

func (r *ControllerWebhook) validateController(ctx context.Context, obj *slinkyv1beta1.Controller) (admission.Warnings, []error) {
	var warns admission.Warnings
	var errs []error

	// Ref: https://slurm.schedmd.com/man_index.html#configuration_files
	denyConfigFiles := []string{
		"slurm.conf",
		"slurmdbd.conf",
	}
	knownConfigFiles := []string{
		"acct_gather.conf",
		"burst_buffer.conf",
		"cgroup.conf",
		"cli_filter.lua",
		"gres.conf",
		"helpers.conf",
		"job_container.conf",
		"job_submit.lua",
		"knl.conf", // deprecated
		"mpi.conf",
		"oci.conf",
		"plugstack.conf",
		"topology.conf",
		"topology.yaml",
	}

	refs := obj.Spec.ConfigFileRefs
	for _, ref := range refs {
		configMap := &corev1.ConfigMap{}
		configMapKey := types.NamespacedName{
			Name:      ref.Name,
			Namespace: obj.Namespace,
		}
		if err := r.Get(ctx, configMapKey, configMap); err != nil {
			errs = append(errs, err)
			continue
		}
		configFiles := structutils.Keys(configMap.Data)
		controllerlog.V(1).Info("configMap files", "files", configFiles)
		for _, file := range configFiles {
			if slices.Contains(denyConfigFiles, file) {
				errs = append(errs, fmt.Errorf("the configFile is reserved for slurm-operator use: %s", file))
			} else if !slices.Contains(knownConfigFiles, file) {
				warns = append(warns, fmt.Sprintf("the configFile is unknown to Slurm, make sure to include it in another config file otherwise it is ignored: %s", file))
			}
		}
	}

	return warns, errs
}
