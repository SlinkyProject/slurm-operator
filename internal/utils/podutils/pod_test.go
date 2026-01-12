// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package podutils

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsRunningAndReady(t *testing.T) {
	var podA, podB corev1.Pod
	podA.Status.Phase = corev1.PodRunning
	podA.Status.Conditions = append(podA.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})
	podB.Status.Phase = corev1.PodFailed
	podB.Status.Conditions = podA.Status.Conditions
	podB.Status.Conditions = append(podB.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionFalse,
	})
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "podA should be Running and Ready",
			args: args{
				pod: &podA,
			},
			want: true,
		},
		{
			name: "podB should not be Running and Ready",
			args: args{
				pod: &podB,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRunningAndReady(tt.args.pod); got != tt.want {
				t.Errorf("IsRunningAndReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsRunningAndReady(b *testing.B) {
	var podA, podB corev1.Pod
	podA.Status.Phase = corev1.PodRunning
	podA.Status.Conditions = append(podA.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})
	podB.Status.Phase = corev1.PodFailed
	podB.Status.Conditions = podA.Status.Conditions
	podB.Status.Conditions = append(podB.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionFalse,
	})
	benchmarks := []struct {
		name string
		pod  *corev1.Pod
	}{
		{
			name: "podA Running and Ready",
			pod:  &podA,
		},
		{
			name: "podB Not Running and Ready",
			pod:  &podB,
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				IsRunningAndReady(bb.pod)
			}
		})
	}
}

func newPod(now metav1.Time, ready bool, beforeSec int) *corev1.Pod {
	conditionStatus := corev1.ConditionFalse
	if ready {
		conditionStatus = corev1.ConditionTrue
	}
	return &corev1.Pod{
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					LastTransitionTime: metav1.NewTime(now.Add(-1 * time.Duration(beforeSec) * time.Second)),
					Status:             conditionStatus,
				},
			},
		},
	}
}

func TestIsRunningAndAvailable(t *testing.T) {
	type args struct {
		pod             *corev1.Pod
		minReadySeconds int32
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Not ready before 0",
			args: args{
				pod:             newPod(metav1.Now(), false, 0),
				minReadySeconds: 0,
			},
			want: false,
		},
		{
			name: "Ready before 0",
			args: args{
				pod:             newPod(metav1.Now(), true, 0),
				minReadySeconds: 1,
			},
			want: false,
		},
		{
			name: "Ready 0",
			args: args{
				pod:             newPod(metav1.Now(), true, 0),
				minReadySeconds: 0,
			},
			want: true,
		},
		{
			name: "Ready after 50",
			args: args{
				pod:             newPod(metav1.Now(), true, 51),
				minReadySeconds: 50,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRunningAndAvailable(tt.args.pod, tt.args.minReadySeconds); got != tt.want {
				t.Errorf("IsRunningAndAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsRunningAndAvailable(b *testing.B) {
	benchmarks := []struct {
		name            string
		pod             *corev1.Pod
		minReadySeconds int32
	}{
		{
			name:            "Not ready before 0",
			pod:             newPod(metav1.Now(), false, 0),
			minReadySeconds: 0,
		},
		{
			name:            "Ready before 0",
			pod:             newPod(metav1.Now(), true, 0),
			minReadySeconds: 1,
		},
		{
			name:            "Ready 0",
			pod:             newPod(metav1.Now(), true, 0),
			minReadySeconds: 0,
		},
		{
			name:            "Ready after 50",
			pod:             newPod(metav1.Now(), true, 51),
			minReadySeconds: 50,
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				IsRunningAndAvailable(bb.pod, bb.minReadySeconds)
			}
		})
	}
}

func TestIsCreated(t *testing.T) {
	var podA, podB corev1.Pod
	podA.Status.Phase = corev1.PodRunning
	podB.Status.Phase = ""
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "podA should not be created",
			args: args{
				pod: &podA,
			},
			want: true,
		},
		{
			name: "podB should not be created",
			args: args{
				pod: &podB,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCreated(tt.args.pod); got != tt.want {
				t.Errorf("IsCreated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsCreated(b *testing.B) {
	var podA, podB corev1.Pod
	podA.Status.Phase = corev1.PodRunning
	podB.Status.Phase = ""
	benchmarks := []struct {
		name string
		pod  *corev1.Pod
	}{
		{
			name: "podA Created",
			pod:  &podA,
		},
		{
			name: "podB Not Created",
			pod:  &podB,
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				IsCreated(bb.pod)
			}
		})
	}
}

func TestIsPending(t *testing.T) {
	var podA, podB corev1.Pod
	podA.Status.Phase = corev1.PodPending
	podB.Status.Phase = ""
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "podA should be Pending",
			args: args{
				pod: &podA,
			},
			want: true,
		},
		{
			name: "podB should not be Pending",
			args: args{
				pod: &podB,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPending(tt.args.pod); got != tt.want {
				t.Errorf("IsPending() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsPending(b *testing.B) {
	var podA, podB corev1.Pod
	podA.Status.Phase = corev1.PodPending
	podB.Status.Phase = ""
	benchmarks := []struct {
		name string
		pod  *corev1.Pod
	}{
		{
			name: "podA Pending",
			pod:  &podA,
		},
		{
			name: "podB Not Pending",
			pod:  &podB,
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				IsPending(bb.pod)
			}
		})
	}
}

func TestIsFailed(t *testing.T) {
	var podA, podB corev1.Pod
	podA.Status.Phase = corev1.PodFailed
	podB.Status.Phase = ""
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "podA should be Failed",
			args: args{
				pod: &podA,
			},
			want: true,
		},
		{
			name: "podB should not be Failed",
			args: args{
				pod: &podB,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFailed(tt.args.pod); got != tt.want {
				t.Errorf("IsFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsFailed(b *testing.B) {
	var podA, podB corev1.Pod
	podA.Status.Phase = corev1.PodFailed
	podB.Status.Phase = ""
	benchmarks := []struct {
		name string
		pod  *corev1.Pod
	}{
		{
			name: "podA Failed",
			pod:  &podA,
		},
		{
			name: "podB Not Failed",
			pod:  &podB,
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				IsFailed(bb.pod)
			}
		})
	}
}

func TestIsSucceeded(t *testing.T) {
	var podA, podB corev1.Pod
	podA.Status.Phase = corev1.PodSucceeded
	podB.Status.Phase = ""
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "podA should be Succeeded",
			args: args{
				pod: &podA,
			},
			want: true,
		},
		{
			name: "podB should not be Succeeded",
			args: args{
				pod: &podB,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSucceeded(tt.args.pod); got != tt.want {
				t.Errorf("IsSucceeded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsSucceeded(b *testing.B) {
	var podA, podB corev1.Pod
	podA.Status.Phase = corev1.PodSucceeded
	podB.Status.Phase = ""
	benchmarks := []struct {
		name string
		pod  *corev1.Pod
	}{
		{
			name: "podA Succeeded",
			pod:  &podA,
		},
		{
			name: "podB Not Succeeded",
			pod:  &podB,
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				IsSucceeded(bb.pod)
			}
		})
	}
}

func TestIsTerminating(t *testing.T) {
	var podA, podB corev1.Pod
	timestamp := metav1.Now()
	podA.SetDeletionTimestamp(&timestamp)
	podB.DeletionTimestamp = nil
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "podA should be Terminating",
			args: args{
				pod: &podA,
			},
			want: true,
		},
		{
			name: "podB should not be Terminating",
			args: args{
				pod: &podB,
			},
			want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTerminating(tt.args.pod); got != tt.want {
				t.Errorf("IsTerminating() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsTerminating(b *testing.B) {
	var podA, podB corev1.Pod
	timestamp := metav1.Now()
	podA.SetDeletionTimestamp(&timestamp)
	podB.DeletionTimestamp = nil
	benchmarks := []struct {
		name string
		pod  *corev1.Pod
	}{
		{
			name: "podA Terminating",
			pod:  &podA,
		},
		{
			name: "podB Not Terminating",
			pod:  &podB,
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				IsTerminating(bb.pod)
			}
		})
	}
}

func TestIsHealthy(t *testing.T) {
	var podA, podB, podC corev1.Pod
	podA.Status.Phase = corev1.PodRunning
	podA.Status.Conditions = append(podA.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})
	podA.DeletionTimestamp = nil
	podB.Status.Phase = corev1.PodFailed
	podB.Status.Conditions = append(podB.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})
	podC.Status.Phase = corev1.PodFailed
	podC.Status.Conditions = append(podC.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})
	timestamp := metav1.Now()
	podC.SetDeletionTimestamp(&timestamp)
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "podA should be Healthy",
			args: args{
				pod: &podA,
			},
			want: true,
		},
		{
			name: "podB should not be Healthy",
			args: args{
				pod: &podB,
			},
			want: false,
		},
		{
			name: "podC should not be Healthy",
			args: args{
				pod: &podC,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHealthy(tt.args.pod); got != tt.want {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsHealthy(b *testing.B) {
	var podA, podB, podC corev1.Pod
	podA.Status.Phase = corev1.PodRunning
	podA.Status.Conditions = append(podA.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})
	podA.DeletionTimestamp = nil
	podB.Status.Phase = corev1.PodFailed
	podB.Status.Conditions = append(podB.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})
	podC.Status.Phase = corev1.PodFailed
	podC.Status.Conditions = append(podC.Status.Conditions, corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	})
	timestamp := metav1.Now()
	podC.SetDeletionTimestamp(&timestamp)
	benchmarks := []struct {
		name string
		pod  *corev1.Pod
	}{
		{
			name: "podA Healthy",
			pod:  &podA,
		},
		{
			name: "podB Not Healthy",
			pod:  &podB,
		},
		{
			name: "podC Not Healthy",
			pod:  &podC,
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				IsHealthy(bb.pod)
			}
		})
	}
}
