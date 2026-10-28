// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

// Package k8sutils holds the Kubernetes helpers that e2e tests share: pod exec
// through the API, pod and Node checks, and NodeSet reads and updates through a
// controller-runtime client.
package k8sutils

import (
	corev1 "k8s.io/api/core/v1"
)

// PodReady reports whether the pod's Ready condition is true.
func PodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// HasUnschedulableToleration reports whether the pod tolerates the NoSchedule
// taint of a cordoned Node.
func HasUnschedulableToleration(pod *corev1.Pod) bool {
	for _, toleration := range pod.Spec.Tolerations {
		if toleration.Key == corev1.TaintNodeUnschedulable && toleration.Effect == corev1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}
