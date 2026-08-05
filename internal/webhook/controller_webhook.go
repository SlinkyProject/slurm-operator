// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"errors"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=slinky.slurm.net,resources=controllers,verbs=delete;create;update

type ControllerWebhook struct {
	client.Client
}

// log is for logging in this package.
var controllerlog = logf.Log.WithName("controller-resource")

func (r *ControllerWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &slinkyv1beta1.Controller{}).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-slinky-slurm-net-v1beta1-controller,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=slinky.slurm.net,resources=controllers,verbs=create;update,versions=v1beta1,name=controller-v1beta1.kb.io,admissionReviewVersions=v1beta1

var _ admission.Validator[*slinkyv1beta1.Controller] = &ControllerWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ControllerWebhook) ValidateCreate(ctx context.Context, controller *slinkyv1beta1.Controller) (admission.Warnings, error) {
	controllerlog.Info("validate create", "controller", klog.KObj(controller))

	warns, errs := r.validateController(ctx, controller)

	// https://slurm.schedmd.com/slurm.conf.html#OPT_ClusterName
	controllerName := controller.ClusterName()
	if len(controllerName) > 40 {
		errs = append(errs, fmt.Errorf("ClusterName exceeds 40 characters (%d): %s",
			len(controllerName), controllerName))
	}

	if err := r.validatePVC(ctx, controller); err != nil {
		errs = append(errs, err)
	}

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ControllerWebhook) ValidateUpdate(ctx context.Context, oldController, newController *slinkyv1beta1.Controller) (admission.Warnings, error) {
	controllerlog.Info("validate update", "newController", klog.KObj(newController))

	warns, errs := r.validateController(ctx, newController)

	if newController.ClusterName() != oldController.ClusterName() {
		errs = append(errs, errors.New("cannot change ClusterName after deployment"))
	}
	if !apiequality.Semantic.DeepEqual(newController.Spec.SlurmKeyRef.LocalObjectReference, oldController.Spec.SlurmKeyRef.LocalObjectReference) {
		errs = append(errs, errors.New("cannot change SlurmKeyRef after deployment"))
	}
	if !apiequality.Semantic.DeepEqual(newController.AuthJwtRef(), oldController.AuthJwtRef()) {
		errs = append(errs, errors.New("the value of JwtKeyRef or JwtHs256KeyRef cannot be modified after deployment"))
	}

	// We use volumeClaimTemplates to handle the controller savestate PVC.
	// StatefulSet does not allow update of that field.
	if !apiequality.Semantic.DeepEqual(newController.Spec.Persistence.Enabled, oldController.Spec.Persistence.Enabled) {
		errs = append(errs, errors.New("cannot change persistence.enabled after deployment"))
	}

	if ptr.Deref(newController.Spec.Persistence.Enabled, defaults.DefaultControllerPersistenceEnabled) &&
		newController.Spec.Persistence.ExistingClaim != oldController.Spec.Persistence.ExistingClaim {
		errs = append(errs, errors.New("cannot change persistence.existingClaim after deployment"))
	}

	if err := r.validatePVC(ctx, newController); err != nil {
		errs = append(errs, err)
	}

	return warns, utilerrors.NewAggregate(errs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ControllerWebhook) ValidateDelete(ctx context.Context, controller *slinkyv1beta1.Controller) (admission.Warnings, error) {
	controllerlog.Info("validate delete", "controller", klog.KObj(controller))

	return nil, nil
}

func (r *ControllerWebhook) validateController(ctx context.Context, controller *slinkyv1beta1.Controller) (admission.Warnings, []error) {
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
		"mpi.conf",
		"namespace.yaml",
		"oci.conf",
		"plugstack.conf",
		"topology.conf",
		"topology.yaml",
	}

	refs := controller.Spec.ConfigFileRefs
	for _, ref := range refs {
		configMap := &corev1.ConfigMap{}
		configMapKey := types.NamespacedName{
			Name:      ref.Name,
			Namespace: controller.Namespace,
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

	// Prevent MitM via CVE-2020-8554
	if controller.Spec.Service.ServiceSpecWrapper.ExternalIPs != nil {
		warns = append(warns, "ExternalIPs may not be set for controller service")
	}

	if err := r.validatePVC(ctx, controller); err != nil {
		errs = append(errs, err)
	}

	return warns, errs
}

func (r *ControllerWebhook) validatePVC(ctx context.Context, controller *slinkyv1beta1.Controller) error {
	if controller.Spec.HighAvailability.Enabled {
		pvc := &corev1.PersistentVolumeClaim{}
		pvcKey := types.NamespacedName{
			Namespace: controller.Namespace,
			Name:      controller.Spec.Persistence.ExistingClaim,
		}
		if err := r.Get(ctx, pvcKey, pvc); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get PVC (%s): %w", klog.KObj(pvc), err)
			}
		} else if !slices.Contains(pvc.Spec.AccessModes, corev1.ReadWriteMany) {
			return fmt.Errorf("slurm high availability (HA) mode requires given PVC (%s) to have access mode 'ReadWriteMany'", klog.KObj(pvc))
		}
	}
	return nil
}
