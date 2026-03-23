// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2014 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package podcontrol

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/apis/core/validation"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PodControlInterface interface {
	kubecontroller.PodControlInterface
	// CreateThisPod creates a new pod according to the Pod object with the specified Object as its controller.
	CreateThisPod(ctx context.Context, pod *corev1.Pod, object runtime.Object) error
}

// RealPodControl is the default implementation of PodControlInterface.
type realPodControl struct {
	client.Client
	recorder events.EventRecorder
}

// CreatePods implements PodControlInterface.
func (r *realPodControl) CreatePods(ctx context.Context, namespace string, template *corev1.PodTemplateSpec, object runtime.Object, controllerRef *metav1.OwnerReference) error {
	return r.CreatePodsWithGenerateName(ctx, namespace, template, object, controllerRef, "")
}

// CreatePodsWithGenerateName implements PodControlInterface.
func (r *realPodControl) CreatePodsWithGenerateName(ctx context.Context, namespace string, template *corev1.PodTemplateSpec, object runtime.Object, controllerRef *metav1.OwnerReference, generateName string) error {
	if err := validateControllerRef(controllerRef); err != nil {
		return err
	}
	pod, err := GetPodFromTemplate(template, object, controllerRef)
	if err != nil {
		return err
	}
	pod.SetNamespace(namespace)
	if len(generateName) > 0 {
		pod.SetGenerateName(generateName)
	}
	return r.createPods(ctx, pod, object)
}

func (r realPodControl) createPods(ctx context.Context, pod *corev1.Pod, object runtime.Object) error {
	if len(labels.Set(pod.Labels)) == 0 {
		return fmt.Errorf("unable to create pods, no labels")
	}
	if err := r.Create(ctx, pod); err != nil {
		// only send an event if the namespace isn't terminating
		if !apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
			r.recorder.Eventf(object, pod, corev1.EventTypeWarning, kubecontroller.FailedCreatePodReason, "Create", "Error creating: %v", err)
		}
		return err
	}

	logger := klog.FromContext(ctx)
	accessor, err := meta.Accessor(object)
	if err != nil {
		logger.Error(err, "parentObject does not have ObjectMeta")
		return nil
	}
	logger.V(4).Info("Controller created pod", "controller", accessor.GetName(), "pod", klog.KObj(pod))
	r.recorder.Eventf(object, pod, corev1.EventTypeNormal, kubecontroller.SuccessfulCreatePodReason, "Create", "Created pod: %v", pod.GetName())

	return nil
}

// CreatePod implements PodControlInterface.
func (r *realPodControl) CreateThisPod(ctx context.Context, pod *corev1.Pod, object runtime.Object) error {
	if pod == nil {
		return fmt.Errorf("pod cannot be nil")
	}
	return r.createPods(ctx, pod, object)
}

// DeletePod implements PodControlInterface.
func (r *realPodControl) DeletePod(ctx context.Context, namespace string, podName string, object runtime.Object) error {
	accessor, err := meta.Accessor(object)
	if err != nil {
		return fmt.Errorf("object does not have ObjectMeta, %w", err)
	}
	logger := klog.FromContext(ctx)
	logger.V(2).Info("Deleting pod", "controller", accessor.GetName(), "pod", klog.KRef(namespace, podName))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
		},
	}
	if err := r.Delete(ctx, pod); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(4).Info("Pod has already been deleted.", "pod", klog.KRef(namespace, podName))
			return err
		}
		r.recorder.Eventf(object, pod, corev1.EventTypeWarning, kubecontroller.FailedDeletePodReason, "Delete", "Error deleting: %v", err)
		return fmt.Errorf("unable to delete pods: %w", err)
	}
	r.recorder.Eventf(object, pod, corev1.EventTypeNormal, kubecontroller.SuccessfulDeletePodReason, "Delete", "Deleted pod: %v", podName)

	return nil
}

// PatchPod implements PodControlInterface.
func (r *realPodControl) PatchPod(ctx context.Context, namespace string, name string, data []byte) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	patch := client.RawPatch(types.StrategicMergePatchType, data)
	return r.Patch(ctx, pod, patch)
}

var _ PodControlInterface = &realPodControl{}

func NewPodControl(client client.Client, recorder events.EventRecorder) PodControlInterface {
	return &realPodControl{
		Client:   client,
		recorder: recorder,
	}
}

func GetPodFromTemplate(template *corev1.PodTemplateSpec, parentObject runtime.Object, controllerRef *metav1.OwnerReference) (*corev1.Pod, error) {
	desiredLabels := getPodsLabelSet(template)
	desiredFinalizers := getPodsFinalizers(template)
	desiredAnnotations := getPodsAnnotationSet(template)
	accessor, err := meta.Accessor(parentObject)
	if err != nil {
		return nil, fmt.Errorf("parentObject does not have ObjectMeta, %w", err)
	}
	prefix := getPodsPrefix(accessor.GetName())

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:       desiredLabels,
			Annotations:  desiredAnnotations,
			GenerateName: prefix,
			Finalizers:   desiredFinalizers,
		},
	}
	if controllerRef != nil {
		pod.OwnerReferences = append(pod.OwnerReferences, *controllerRef)
	}
	pod.Spec = *template.Spec.DeepCopy()
	return pod, nil
}

func getPodsLabelSet(template *corev1.PodTemplateSpec) labels.Set {
	desiredLabels := make(labels.Set)
	for k, v := range template.Labels {
		desiredLabels[k] = v
	}
	return desiredLabels
}

func getPodsFinalizers(template *corev1.PodTemplateSpec) []string {
	desiredFinalizers := make([]string, len(template.Finalizers))
	copy(desiredFinalizers, template.Finalizers)
	return desiredFinalizers
}

func getPodsAnnotationSet(template *corev1.PodTemplateSpec) labels.Set {
	desiredAnnotations := make(labels.Set)
	for k, v := range template.Annotations {
		desiredAnnotations[k] = v
	}
	return desiredAnnotations
}

func getPodsPrefix(controllerName string) string {
	// use the dash (if the name isn't too long) to make the pod name a bit prettier
	prefix := fmt.Sprintf("%s-", controllerName)
	if len(validation.ValidatePodName(prefix, true)) != 0 {
		prefix = controllerName
	}
	return prefix
}

func validateControllerRef(controllerRef *metav1.OwnerReference) error {
	if controllerRef == nil {
		return fmt.Errorf("controllerRef is nil")
	}
	if len(controllerRef.APIVersion) == 0 {
		return fmt.Errorf("controllerRef has empty APIVersion")
	}
	if len(controllerRef.Kind) == 0 {
		return fmt.Errorf("controllerRef has empty Kind")
	}
	if controllerRef.Controller == nil || !*controllerRef.Controller {
		return fmt.Errorf("controllerRef.Controller is not set to true")
	}
	if controllerRef.BlockOwnerDeletion == nil || !*controllerRef.BlockOwnerDeletion {
		return fmt.Errorf("controllerRef.BlockOwnerDeletion is not set to true")
	}
	return nil
}
