// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2014 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/utils/mathutils"
	"github.com/togethercomputer/slurm-operator/internal/utils/podutils"
	"github.com/togethercomputer/slurm-operator/internal/utils/structutils"
)

// ActivePods type allows custom sorting of pods so a controller can pick the best ones to delete.
type ActivePods []*corev1.Pod

func (o ActivePods) Len() int {
	return len(o)
}

func (o ActivePods) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

// Less compares two pods and returns true if the first one should be preferred for deletion.
func (o ActivePods) Less(i, j int) bool {
	pod1 := o[i]
	pod2 := o[j]

	// Step: unassigned < assigned
	// If only one of the pods is unassigned, the unassigned one is smaller
	if pod1.Spec.NodeName != pod2.Spec.NodeName && (len(pod1.Spec.NodeName) == 0 || len(pod2.Spec.NodeName) == 0) {
		return len(pod1.Spec.NodeName) == 0
	}

	// Step: PodPending < PodUnknown < PodRunning
	podPhaseToWeight := map[corev1.PodPhase]int{corev1.PodPending: 0, corev1.PodUnknown: 1, corev1.PodRunning: 2}
	if podPhaseToWeight[pod1.Status.Phase] != podPhaseToWeight[pod2.Status.Phase] {
		return podPhaseToWeight[pod1.Status.Phase] < podPhaseToWeight[pod2.Status.Phase]
	}

	// Step: not ready < ready
	// If only one of the pods is not ready, the not ready one is smaller
	if podutil.IsPodReady(pod1) != podutil.IsPodReady(pod2) {
		return !podutil.IsPodReady(pod1)
	}

	// Step: lower pod-deletion-cost < higher pod-deletion-cost
	podDeletionCost1, _ := structutils.GetNumberFromAnnotations(pod1.Annotations, slinkyv1beta1.AnnotationPodDeletionCost)
	podDeletionCost2, _ := structutils.GetNumberFromAnnotations(pod2.Annotations, slinkyv1beta1.AnnotationPodDeletionCost)
	if podDeletionCost1 != podDeletionCost2 {
		return podDeletionCost1 < podDeletionCost2
	}

	// Step: earlier deadline timestamp < later deadline timestamp
	podDeadline1, _ := structutils.GetTimeFromAnnotations(pod1.Annotations, slinkyv1beta1.AnnotationPodDeadline)
	podDeadline2, _ := structutils.GetTimeFromAnnotations(pod2.Annotations, slinkyv1beta1.AnnotationPodDeadline)
	if !podDeadline1.Equal(podDeadline2) {
		return podDeadline1.Before(podDeadline2)
	}

	// Step: cordon < not cordon
	podCordon1, _ := structutils.GetBoolFromAnnotations(pod1.Annotations, slinkyv1beta1.AnnotationPodCordon)
	podCordon2, _ := structutils.GetBoolFromAnnotations(pod2.Annotations, slinkyv1beta1.AnnotationPodCordon)
	if podCordon1 || podCordon2 {
		return podCordon1
	}

	// Step: higher ordinal < lower ordinal
	if GetOrdinal(pod1) != GetOrdinal(pod2) {
		return GetOrdinal(pod1) > GetOrdinal(pod2)
	}

	// TODO: take availability into account when we push minReadySeconds information from nodeset into pods,
	//       see https://github.com/kubernetes/kubernetes/issues/22065
	// Step: Been ready for empty time < less time < more time
	// If both pods are ready, the latest ready one is smaller
	if podutil.IsPodReady(pod1) && podutil.IsPodReady(pod2) {
		readyTime1 := podReadyTime(pod1)
		readyTime2 := podReadyTime(pod2)
		if !readyTime1.Equal(readyTime2) {
			return afterOrZero(readyTime1.Time, readyTime2.Time)
		}
	}

	// Step: Empty creation time pods < newer pods < older pods
	if !pod1.CreationTimestamp.Equal(&pod2.CreationTimestamp) {
		return afterOrZero(pod1.CreationTimestamp.Time, pod2.CreationTimestamp.Time)
	}

	return false
}

func podReadyTime(pod *corev1.Pod) *metav1.Time {
	if podutil.IsPodReady(pod) {
		for _, c := range pod.Status.Conditions {
			// we only care about pod ready conditions
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				return &c.LastTransitionTime
			}
		}
	}
	return &metav1.Time{}
}

// afterOrZero checks if time t1 is after time t2; if one of them
// is zero, the zero time is seen as after non-zero time.
func afterOrZero(t1, t2 time.Time) bool {
	if t1.IsZero() || t2.IsZero() {
		return t1.IsZero()
	}
	return t1.After(t2)
}

// SplitActivePods returns two list of pods partitioned by a number.
func SplitActivePods(pods []*corev1.Pod, partition int) (pods1, pods2 []*corev1.Pod) {
	pivot := mathutils.Clamp(partition, 0, len(pods))

	pods1 = make([]*corev1.Pod, pivot)
	pods2 = make([]*corev1.Pod, len(pods)-pivot)

	sort.Sort(ActivePods(pods))
	copy(pods1, pods[:pivot])
	copy(pods2, pods[pivot:])

	return pods1, pods2
}

// PodsByCreationTimestamp sorts a list of Pods by creation timestamp, using their names as a tie breaker.
type PodsByCreationTimestamp []*corev1.Pod

func (o PodsByCreationTimestamp) Len() int {
	return len(o)
}

func (o PodsByCreationTimestamp) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

func (o PodsByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}

// SplitUnhealthyPods returns lists of healthy and unhealthy pods.
func SplitUnhealthyPods(pods []*corev1.Pod) (unhealthyPods, healthyPods []*corev1.Pod) {
	var numUnhealthy int
	for _, pod := range pods {
		if !podutils.IsHealthy(pod) {
			numUnhealthy++
		}
	}

	unhealthyPods = make([]*corev1.Pod, numUnhealthy)
	healthyPods = make([]*corev1.Pod, len(pods)-numUnhealthy)

	sort.Sort(PodsByCreationTimestamp(pods))
	copy(unhealthyPods, pods[:numUnhealthy])
	copy(healthyPods, pods[numUnhealthy:])

	return unhealthyPods, healthyPods
}
