// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package podutils

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

// CondemnSettleWindow is the minimum duration a newly condemned pod must remain
// condemned before its drained deadline is trusted for candidate evaluation.
//
// This value needs to be greater than or equal to the async cache refresh frequency.
const CondemnSettleWindow = 1 * time.Minute

// IsPodCordon returns true if and only if the cordon annotation is set to true.
func IsPodCordon(pod *corev1.Pod) bool {
	b, _ := structutils.GetBoolFromAnnotations(pod.GetAnnotations(), slinkyv1beta1.AnnotationPodCordon)
	return b
}

// IsPodCordonRequested returns true if and only if the cordon-requested annotation is set to true.
func IsPodCordonRequested(pod *corev1.Pod) bool {
	b, _ := structutils.GetBoolFromAnnotations(pod.GetAnnotations(), slinkyv1beta1.AnnotationPodCordonRequested)
	return b
}

// IsPodCondemned returns true if the pod is marked as a scale-in / rolling-update deletion candidate
// (either as a boolean "true" or as an RFC3339 timestamp).
func IsPodCondemned(pod *corev1.Pod) bool {
	if b, _ := structutils.GetBoolFromAnnotations(pod.GetAnnotations(), slinkyv1beta1.AnnotationPodCondemned); b {
		return true
	}
	t, _ := structutils.GetTimeFromAnnotations(pod.GetAnnotations(), slinkyv1beta1.AnnotationPodCondemned)
	return !t.IsZero()
}

// IsPodCondemnedSettled returns true if the pod is condemned and at least
// CondemnSettleWindow has elapsed since it was condemned (or the annotation is set to "true").
func IsPodCondemnedSettled(pod *corev1.Pod, now time.Time) bool {
	if b, _ := structutils.GetBoolFromAnnotations(pod.GetAnnotations(), slinkyv1beta1.AnnotationPodCondemned); b {
		return true
	}
	t, _ := structutils.GetTimeFromAnnotations(pod.GetAnnotations(), slinkyv1beta1.AnnotationPodCondemned)
	return !t.IsZero() && !now.Before(t.Add(CondemnSettleWindow))
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
