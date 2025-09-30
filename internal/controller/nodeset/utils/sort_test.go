// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2015 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"math/rand/v2"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestSortingActivePods(t *testing.T) {
	now := metav1.Now()
	then := metav1.Time{Time: now.AddDate(0, -1, 0)}

	tests := []struct {
		name      string
		pods      []corev1.Pod
		wantOrder []string
	}{
		{
			name: "Sorts by active pod",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "unscheduled"},
					Spec:       corev1.PodSpec{NodeName: ""},
					Status:     corev1.PodStatus{Phase: corev1.PodPending},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "scheduledButPending"},
					Spec:       corev1.PodSpec{NodeName: "bar"},
					Status:     corev1.PodStatus{Phase: corev1.PodPending},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "unknownPhase"},
					Spec:       corev1.PodSpec{NodeName: "foo"},
					Status:     corev1.PodStatus{Phase: corev1.PodUnknown},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "runningButNotReady"},
					Spec:       corev1.PodSpec{NodeName: "foo"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "runningNoLastTransitionTime"},
					Spec:       corev1.PodSpec{NodeName: "foo"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "runningWithLastTransitionTime"},
					Spec:       corev1.PodSpec{NodeName: "foo"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: now},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "runningLongerTime"},
					Spec:       corev1.PodSpec{NodeName: "foo"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: then},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "oldest", CreationTimestamp: then},
					Spec:       corev1.PodSpec{NodeName: "foo"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: then},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "runningWithCost",
						Annotations: map[string]string{slinkyv1beta1.AnnotationPodDeletionCost: "1"},
					},
					Spec: corev1.PodSpec{NodeName: "foo"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "runningWithCordon",
						Annotations: map[string]string{slinkyv1beta1.AnnotationPodCordon: "True"},
					},
					Spec: corev1.PodSpec{NodeName: "foo"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "runningWithOrdinal-1"},
					Spec:       corev1.PodSpec{NodeName: "foo"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "runningWithDeadline",
						Annotations: map[string]string{slinkyv1beta1.AnnotationPodDeadline: time.Now().Format(time.RFC3339)},
					},
					Spec: corev1.PodSpec{NodeName: "foo"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
			},
			wantOrder: []string{
				"unscheduled",
				"scheduledButPending",
				"unknownPhase",
				"runningButNotReady",
				"runningWithCordon",
				"runningWithOrdinal-1",
				"runningNoLastTransitionTime",
				"runningWithLastTransitionTime",
				"runningLongerTime",
				"oldest",
				"runningWithDeadline",
				"runningWithCost",
			},
		},
		{
			name: "Sort ordinals",
			pods: []corev1.Pod{
				newRunningPod("ordinal-0", nil),
				newRunningPod("ordinal-1", nil),
				newRunningPod("ordinal-2", nil),
			},
			wantOrder: []string{
				"ordinal-2",
				"ordinal-1",
				"ordinal-0",
			},
		},
		{
			name: "Sort cordon",
			pods: []corev1.Pod{
				newRunningPod("regular", nil),
				newRunningPod("podCordon", map[string]string{
					slinkyv1beta1.AnnotationPodCordon: "True",
				}),
			},
			wantOrder: []string{
				"podCordon",
				"regular",
			},
		},
		{
			name: "Sort deadlines",
			pods: []corev1.Pod{
				newRunningPod("noDeadline", nil),
				newRunningPod("deadlineBefore", map[string]string{
					slinkyv1beta1.AnnotationPodDeadline: time.Now().Add(-time.Hour).Format(time.RFC3339),
				}),
				newRunningPod("deadlineNow", map[string]string{
					slinkyv1beta1.AnnotationPodDeadline: time.Now().Format(time.RFC3339),
				}),
				newRunningPod("deadlineLater", map[string]string{
					slinkyv1beta1.AnnotationPodDeadline: time.Now().Add(time.Hour).Format(time.RFC3339),
				}),
			},
			wantOrder: []string{
				"noDeadline",
				"deadlineBefore",
				"deadlineNow",
				"deadlineLater",
			},
		},
		{
			name: "Sort deletion cost",
			pods: []corev1.Pod{
				newRunningPod("costNeg10", map[string]string{
					slinkyv1beta1.AnnotationPodDeletionCost: "-10",
				}),
				newRunningPod("cost0", nil),
				newRunningPod("costPos10", map[string]string{
					slinkyv1beta1.AnnotationPodDeletionCost: "10",
				}),
			},
			wantOrder: []string{
				"costNeg10",
				"cost0",
				"costPos10",
			},
		},
		{
			name: "Sort mixed",
			pods: []corev1.Pod{
				newRunningPod("ordinal-0", nil),
				newRunningPod("ordinal-1", nil),
				newRunningPod("deadlineNow", map[string]string{
					slinkyv1beta1.AnnotationPodDeadline: time.Now().Format(time.RFC3339),
				}),
				newRunningPod("deadlineLater", map[string]string{
					slinkyv1beta1.AnnotationPodDeadline: time.Now().Add(time.Hour).Format(time.RFC3339),
				}),
				newRunningPod("podCordoned", map[string]string{
					slinkyv1beta1.AnnotationPodCordon: "True",
				}),
				newRunningPod("podCordonedAndDeadlineNow", map[string]string{
					slinkyv1beta1.AnnotationPodCordon:   "True",
					slinkyv1beta1.AnnotationPodDeadline: time.Now().Format(time.RFC3339),
				}),
				newRunningPod("podCordonedAndDeadlineLater", map[string]string{
					slinkyv1beta1.AnnotationPodCordon:   "True",
					slinkyv1beta1.AnnotationPodDeadline: time.Now().Add(time.Hour).Format(time.RFC3339),
				}),
				newRunningPod("deletionCostNeg10", map[string]string{
					slinkyv1beta1.AnnotationPodDeletionCost: "-10",
				}),
				newRunningPod("deletionCostPos10", map[string]string{
					slinkyv1beta1.AnnotationPodDeletionCost: "1",
				}),
				newRunningPod("cordonedDeadlineCost100", map[string]string{
					slinkyv1beta1.AnnotationPodCordon:       "True",
					slinkyv1beta1.AnnotationPodDeadline:     time.Now().Format(time.RFC3339),
					slinkyv1beta1.AnnotationPodDeletionCost: "100",
				}),
			},
			wantOrder: []string{
				"deletionCostNeg10",
				"podCordoned",
				"ordinal-1",
				"ordinal-0",
				"podCordonedAndDeadlineNow",
				"deadlineNow",
				"podCordonedAndDeadlineLater",
				"deadlineLater",
				"deletionCostPos10",
				"cordonedDeadlineCost100",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			numPods := len(test.pods)

			for range 20 {
				idx := rand.Perm(numPods)
				randomizedPods := make([]*corev1.Pod, numPods)
				for j := range numPods {
					randomizedPods[j] = &test.pods[idx[j]]
				}

				sort.Sort(ActivePods(randomizedPods))
				gotOrder := make([]string, len(randomizedPods))
				for i := range randomizedPods {
					gotOrder[i] = randomizedPods[i].Name
				}

				if diff := cmp.Diff(test.wantOrder, gotOrder); diff != "" {
					t.Errorf("Sorted active pod names (-want,+got):\n%s", diff)
				}
			}
		})
	}
}

func newRunningPod(name string, annotations map[string]string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{NodeName: "foo"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
}

func Test_afterOrZero(t *testing.T) {
	type args struct {
		t1 time.Time
		t2 time.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "unit time",
			args: args{
				t1: time.Time{},
				t2: time.Time{},
			},
			want: true,
		},
		{
			name: "equal, not zero",
			args: args{
				t1: time.Unix(0, 0),
				t2: time.Unix(0, 0),
			},
			want: false,
		},
		{
			name: "after",
			args: args{
				t1: time.Unix(10, 0),
				t2: time.Unix(0, 0),
			},
			want: true,
		},
		{
			name: "before",
			args: args{
				t1: time.Unix(0, 0),
				t2: time.Unix(10, 0),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := afterOrZero(tt.args.t1, tt.args.t2); got != tt.want {
				t.Errorf("afterOrZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitActivePods(t *testing.T) {
	type args struct {
		pods      []*corev1.Pod
		partition int
	}
	tests := []struct {
		name           string
		args           args
		wantPods1Names []string
		wantPods2Names []string
	}{
		{
			name: "Empty",
			args: args{
				pods:      nil,
				partition: 0,
			},
			wantPods1Names: []string{},
			wantPods2Names: []string{},
		},
		{
			name: "Partition 0",
			args: args{
				pods: []*corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "foo-0"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "foo-1"}},
				},
				partition: 0,
			},
			wantPods1Names: []string{},
			wantPods2Names: []string{"foo-1", "foo-0"},
		},
		{
			name: "Partition 1",
			args: args{
				pods: []*corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "foo-0"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "foo-1"}},
				},
				partition: 1,
			},
			wantPods1Names: []string{"foo-1"},
			wantPods2Names: []string{"foo-0"},
		},
		{
			name: "Partition 2",
			args: args{
				pods: []*corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "foo-0"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "foo-1"}},
				},
				partition: 2,
			},
			wantPods1Names: []string{"foo-1", "foo-0"},
			wantPods2Names: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPods1, gotPods2 := SplitActivePods(tt.args.pods, tt.args.partition)

			gotPods1Names := make([]string, len(gotPods1))
			for i := range gotPods1 {
				gotPods1Names[i] = gotPods1[i].Name
			}
			gotPods2Names := make([]string, len(gotPods2))
			for i := range gotPods2 {
				gotPods2Names[i] = gotPods2[i].Name
			}

			if diff := cmp.Diff(tt.wantPods1Names, gotPods1Names); diff != "" {
				t.Errorf("Sorted active pod names (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantPods2Names, gotPods2Names); diff != "" {
				t.Errorf("Sorted active pod names (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestSplitUnhealthyPods(t *testing.T) {
	type args struct {
		pods []*corev1.Pod
	}
	tests := []struct {
		name              string
		args              args
		wantUnhealthyPods []*corev1.Pod
		wantHealthyPods   []*corev1.Pod
	}{
		{
			name: "empty",
			args: args{
				pods: nil,
			},
			wantUnhealthyPods: []*corev1.Pod{},
			wantHealthyPods:   []*corev1.Pod{},
		},
		{
			name: "mixed",
			args: args{
				pods: []*corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pod1"},
						Status:     corev1.PodStatus{Phase: corev1.PodPending},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "pod2"},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{Type: corev1.PodReady, Status: corev1.ConditionTrue},
							},
						},
					},
				},
			},
			wantUnhealthyPods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pod1"},
					Status:     corev1.PodStatus{Phase: corev1.PodPending},
				},
			},
			wantHealthyPods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pod2"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUnhealthyPods, gotHealthyPods := SplitUnhealthyPods(tt.args.pods)
			if !apiequality.Semantic.DeepEqual(gotUnhealthyPods, tt.wantUnhealthyPods) {
				t.Errorf("SplitUnhealthyPods() gotUnhealthyPods = %v, want %v", gotUnhealthyPods, tt.wantUnhealthyPods)
			}
			if !apiequality.Semantic.DeepEqual(gotHealthyPods, tt.wantHealthyPods) {
				t.Errorf("SplitUnhealthyPods() gotHealthyPods = %v, want %v", gotHealthyPods, tt.wantHealthyPods)
			}
		})
	}
}
