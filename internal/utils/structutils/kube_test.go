// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package structutils_test

import (
	"testing"

	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_strategicMergePatch(t *testing.T) {
	test_strategicMergePatch_pod(t)
}

func test_strategicMergePatch_pod(t *testing.T) {
	tests := []struct {
		name  string
		base  *corev1.Pod
		patch *corev1.Pod
		want  *corev1.Pod
	}{
		{
			name:  "all nil",
			base:  nil,
			patch: nil,
			want:  nil,
		},
		{
			name:  "patch nil",
			base:  &corev1.Pod{},
			patch: nil,
			want:  &corev1.Pod{},
		},
		{
			name:  "base nil",
			base:  nil,
			patch: &corev1.Pod{},
			want:  &corev1.Pod{},
		},
		{
			name: "mixed data",
			base: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "foo",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "foo",
							Image: "foo",
							Args:  []string{"--opt"},
						},
					},
				},
			},
			patch: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"bar": "bar",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "foo",
							Image: "foo",
							Args:  []string{"--opt2"},
						},
						{
							Name:  "bar",
							Image: "bar",
						},
					},
				},
			},
			want: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"foo": "foo",
						"bar": "bar",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "foo",
							Image: "foo",
							Args:  []string{"--opt2"},
						},
						{
							Name:  "bar",
							Image: "bar",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := structutils.StrategicMergePatch(tt.base, tt.patch)
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("StrategicMergePatch() = %v, want %v", got, tt.want)
			}
		})
	}
}
