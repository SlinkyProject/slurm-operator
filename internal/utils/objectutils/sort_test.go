// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSortingActivePods(t *testing.T) {
	tests := []struct {
		name         string
		pods         []corev1.Pod
		wantPodNames []string
	}{
		{
			name: "in order",
			pods: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Namespace: corev1.NamespaceDefault, Name: "pod-0"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: corev1.NamespaceDefault, Name: "pod-1"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: corev1.NamespaceDefault, Name: "pod-2"}},
			},
			wantPodNames: []string{
				"pod-0",
				"pod-1",
				"pod-2",
			},
		},
		{
			name: "reverse order",
			pods: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Namespace: corev1.NamespaceDefault, Name: "pod-2"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: corev1.NamespaceDefault, Name: "pod-1"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: corev1.NamespaceDefault, Name: "pod-0"}},
			},
			wantPodNames: []string{
				"pod-0",
				"pod-1",
				"pod-2",
			},
		},
		{
			name: "out of order",
			pods: []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Namespace: corev1.NamespaceDefault, Name: "pod-1"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: corev1.NamespaceDefault, Name: "pod-0"}},
				{ObjectMeta: metav1.ObjectMeta{Namespace: corev1.NamespaceDefault, Name: "pod-2"}},
			},
			wantPodNames: []string{
				"pod-0",
				"pod-1",
				"pod-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sort.Sort(PodsByName(tt.pods))
			podNames := make([]string, 0, len(tt.pods))
			for _, pod := range tt.pods {
				podNames = append(podNames, pod.Name)
			}
			require.ElementsMatch(t, podNames, tt.wantPodNames)
		})
	}
}
