// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package structutils_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func Test_strategicMergePatch(t *testing.T) {
	test_strategicMergePatch_pod(t)
	test_strategicMergePatch_explicitZeroValues(t)
}

// Explicitly-set zero values (false, 0, and a zero quantity) are meaningful in
// the Kubernetes API and must survive the merge: `allowPrivilegeEscalation:
// false` is required by the restricted Pod Security Standard, `privileged:
// false` is the only way to override a `true` in the base spec, and
// `runAsUser: 0` is an explicit request for root.
func test_strategicMergePatch_explicitZeroValues(t *testing.T) {
	base := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "app",
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To(true),
					},
				},
			},
		},
	}
	patch := &corev1.Pod{
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken:  ptr.To(false),
			TerminationGracePeriodSeconds: ptr.To(int64(0)),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: ptr.To(int64(0)),
			},
			Containers: []corev1.Container{
				{
					Name: "app",
					SecurityContext: &corev1.SecurityContext{
						Privileged:               ptr.To(false),
						AllowPrivilegeEscalation: ptr.To(false),
					},
				},
			},
		},
	}

	got := structutils.StrategicMergePatch(base, patch)

	require.Equal(t, ptr.To(false), got.Spec.AutomountServiceAccountToken)
	require.Equal(t, ptr.To(int64(0)), got.Spec.TerminationGracePeriodSeconds)
	require.NotNil(t, got.Spec.SecurityContext)
	require.Equal(t, ptr.To(int64(0)), got.Spec.SecurityContext.RunAsUser)
	require.Len(t, got.Spec.Containers, 1)
	sc := got.Spec.Containers[0].SecurityContext
	require.NotNil(t, sc)
	require.Equal(t, ptr.To(false), sc.AllowPrivilegeEscalation)
	require.Equal(t, ptr.To(false), sc.Privileged)
	// Fields only present in the base are untouched by the patch.
	require.Equal(t, "app", got.Spec.Containers[0].Image)
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
			require.Equal(t, tt.want, structutils.StrategicMergePatch(tt.base, tt.patch))
		})
	}
}
