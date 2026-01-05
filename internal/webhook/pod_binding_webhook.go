// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
)

type PodBindingWebhook struct {
	client.Client
}

// log is for logging in this package.
var bindinglog = logf.Log.WithName("binding-resource")

func (r *PodBindingWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Binding{}).
		WithDefaulter(r).
		Complete()
}

// +kubebuilder:rbac:groups="",resources=node,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;update;patch;watch
// +kubebuilder:rbac:groups="",resources=pods/binding,verbs=get;list;watch
// +kubebuilder:webhook:path=/mutate--v1-binding,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups="",resources=pods/binding,verbs=create,versions=v1,name=podsbinding-v1.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &PodBindingWebhook{}

// Default implements admission.CustomDefaulter.
func (r *PodBindingWebhook) Default(ctx context.Context, obj runtime.Object) error {
	binding, ok := obj.(*corev1.Binding)
	if !ok {
		return fmt.Errorf("expected a Binding but got a %T", obj)
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

	node := &corev1.Node{}
	nodeKey := types.NamespacedName{Name: binding.Target.Name}
	if err := r.Get(ctx, nodeKey, node); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	topologyLine := node.Annotations[slinkyv1beta1.AnnotationNodeTopologyLine]

	toUpdate := pod.DeepCopy()
	toUpdate.Annotations[slinkyv1beta1.AnnotationNodeTopologyLine] = topologyLine
	if err := r.Patch(ctx, toUpdate, client.StrategicMergeFrom(pod)); err != nil {
		bindinglog.Error(err, "failed to patch pod annotations", "pod", klog.KObj(pod))
		return err
	}

	bindinglog.Info("updated binding for pod on node", "pod", binding.Name, "node", binding.Target.Name)

	return nil
}
