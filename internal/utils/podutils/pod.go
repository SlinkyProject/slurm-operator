// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package podutils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// IsPodCordon returns true if and only if the delete annotation is nodeset to true.
func IsPodCordon(pod *corev1.Pod) bool {
	return pod.GetAnnotations()[slinkyv1beta1.AnnotationPodCordon] == "true"
}

// IsPodCordonedByNode returns true if the pod's cordon was propagated by the operator from its Kubernetes node.
func IsPodCordonedByNode(pod *corev1.Pod) bool {
	return pod.GetAnnotations()[slinkyv1beta1.AnnotationPodCordonSource] == slinkyv1beta1.PodCordonSourceNode
}

// IsPodCordonedByOperator returns true if the pod's cordon was set by the operator itself, for any reason.
func IsPodCordonedByOperator(pod *corev1.Pod) bool {
	switch pod.GetAnnotations()[slinkyv1beta1.AnnotationPodCordonSource] {
	case slinkyv1beta1.PodCordonSourceNode, slinkyv1beta1.PodCordonSourceTermination:
		return true
	default:
		return false
	}
}

// isRunningAndReady returns true if pod is in the PodRunning Phase, if it has a condition of PodReady.
func IsRunningAndReady(pod *corev1.Pod) bool {
	return IsRunning(pod) && podutil.IsPodReady(pod)
}

// isRunning returns true if pod is in the PodRunning Phase.
func IsRunning(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning
}

func IsRunningAndAvailable(pod *corev1.Pod, minReadySeconds int32) bool {
	return podutil.IsPodAvailable(pod, minReadySeconds, metav1.Now())
}

// isCreated returns true if pod has been created and is maintained by the API server
func IsCreated(pod *corev1.Pod) bool {
	return pod.Status.Phase != ""
}

// isPending returns true if pod has a Phase of PodPending
func IsPending(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodPending
}

// isFailed returns true if pod has a Phase of PodFailed
func IsFailed(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodFailed
}

// isSucceeded returns true if pod has a Phase of PodSucceeded
func IsSucceeded(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded
}

// isTerminating returns true if pod's DeletionTimestamp has been set
func IsTerminating(pod *corev1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

// isHealthy returns true if pod is running and ready and has not been terminated
func IsHealthy(pod *corev1.Pod) bool {
	return IsRunningAndReady(pod) && !IsTerminating(pod)
}
