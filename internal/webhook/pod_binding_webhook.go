// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

type PodBindingWebhook struct {
	client.Client
}

// log is for logging in this package.
var bindinglog = logf.Log.WithName("binding-resource")

func (r *PodBindingWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &corev1.Binding{}).
		WithDefaulter(r).
		Complete()
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;update;patch;watch
// +kubebuilder:rbac:groups="",resources=pods/binding,verbs=get;list;watch
// +kubebuilder:webhook:path=/mutate--v1-binding,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=NoneOnDryRun,groups="",resources=pods/binding,verbs=create,versions=v1,name=podsbinding-v1.kb.io,admissionReviewVersions=v1

var _ admission.Defaulter[*corev1.Binding] = &PodBindingWebhook{}

// Default implements admission.CustomDefaulter.
func (r *PodBindingWebhook) Default(ctx context.Context, binding *corev1.Binding) error {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return fmt.Errorf("get admission request from context: %w", err)
	}

	if req.DryRun != nil && *req.DryRun {
		bindinglog.Info("skipping binding mutation for dry-run request", "pod", binding.Name, "node", binding.Target.Name)
		return nil
	}

	bindinglog.Info("mutate binding for pod on node", "pod", binding.Name, "node", binding.Target.Name)

	pod := &corev1.Pod{}
	podKey := client.ObjectKeyFromObject(binding)
	if err := r.Get(ctx, podKey, pod); err != nil {
		return fmt.Errorf("could not fetch pod for binding: %w", err)
	}

	podLabels := pod.GetLabels()
	if len(podLabels) == 0 || podLabels[labels.AppLabel] != labels.WorkerApp {
		bindinglog.V(1).Info("ignoring pod", "pod", klog.KObj(pod))
		return nil
	}
	nodeNamed := podLabels[slinkyv1beta1.LabelNodeSetSlurmNodeNameMode] == string(slinkyv1beta1.SlurmNodeNameModeKubernetesNode)
	if nodeNamed && pod.Spec.NodeName != "" {
		return nil
	}

	node := &corev1.Node{}
	nodeKey := types.NamespacedName{Name: binding.Target.Name}
	if err := r.Get(ctx, nodeKey, node); err != nil {
		if apierrors.IsNotFound(err) && !nodeNamed {
			return nil
		}
		return err
	}

	topologySpec := node.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec]
	slurmName := nodesetutils.GetDaemonSetPodHostname(node.Name, node.Annotations[slinkyv1beta1.AnnotationNodeHostnameOverride])
	if nodeNamed {
		if problems := utilvalidation.IsDNS1123Label(slurmName); len(problems) != 0 {
			return fmt.Errorf("slurm node name %q is not a valid hostname: %v", slurmName, problems)
		}
	}
	mutateFn := func(pod *corev1.Pod) error {
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec] = topologySpec
		if nodeNamed {
			pod.Labels[slinkyv1beta1.LabelNodeSetPodHostname] = slurmName
		}
		return nil
	}
	if err := objectutils.PatchObject(r.Client, ctx, pod, mutateFn); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		bindinglog.Error(err, "failed to patch pod annotations", "pod", klog.KObj(pod))
		return err
	}

	bindinglog.Info("updated binding for pod on node", "pod", binding.Name, "node", binding.Target.Name)

	return nil
}
