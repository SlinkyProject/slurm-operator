// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package podutils

import (
	corev1 "k8s.io/api/core/v1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
)

// IsPodCordon returns true if and only if the delete annotation is nodeset to true.
func IsPodCordon(pod *corev1.Pod) bool {
	return pod.GetAnnotations()[slinkyv1alpha1.AnnotationPodCordon] == "true"
}

// GetSlurmNodeDrainState returns the current drain state of the underlying Slurm node.
func GetSlurmNodeDrainState(pod *corev1.Pod) string {
	return pod.GetAnnotations()[slinkyv1alpha1.AnnotationSlurmNodeDrainState]
}

// GetSlurmNodeDrainReason returns the reason why the underlying Slurm node was drained.
func GetSlurmNodeDrainReason(pod *corev1.Pod) string {
	return pod.GetAnnotations()[slinkyv1alpha1.AnnotationSlurmNodeDrainReason]
}

// isRunningAndReady returns true if pod is in the PodRunning Phase, if it has a condition of PodReady.
func IsRunningAndReady(pod *corev1.Pod) bool {
	return IsRunning(pod) && podutil.IsPodReady(pod)
}

// isRunning returns true if pod is in the PodRunning Phase.
func IsRunning(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning
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
