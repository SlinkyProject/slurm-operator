// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package k8sutils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func newClient(t *testing.T, objects ...crclient.Object) crclient.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, slinkyv1beta1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func TestPodReady(t *testing.T) {
	tests := []struct {
		name       string
		conditions []corev1.PodCondition
		want       bool
	}{
		{name: "no conditions"},
		{name: "ready", conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}, want: true},
		{name: "not ready", conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}},
		{name: "only scheduled", conditions: []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionTrue}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{Status: corev1.PodStatus{Conditions: tt.conditions}}
			require.Equal(t, tt.want, PodReady(pod))
		})
	}
}

func TestHasUnschedulableToleration(t *testing.T) {
	tests := []struct {
		name        string
		tolerations []corev1.Toleration
		want        bool
	}{
		{name: "no tolerations"},
		{
			name:        "unschedulable NoSchedule",
			tolerations: []corev1.Toleration{{Key: corev1.TaintNodeUnschedulable, Effect: corev1.TaintEffectNoSchedule}},
			want:        true,
		},
		{
			name:        "unschedulable NoExecute",
			tolerations: []corev1.Toleration{{Key: corev1.TaintNodeUnschedulable, Effect: corev1.TaintEffectNoExecute}},
		},
		{
			name:        "other key",
			tolerations: []corev1.Toleration{{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{Spec: corev1.PodSpec{Tolerations: tt.tolerations}}
			require.Equal(t, tt.want, HasUnschedulableToleration(pod))
		})
	}
}

func TestSetNodeUnschedulable(t *testing.T) {
	ctx := context.Background()
	crClient := newClient(t, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-0"}})

	for _, unschedulable := range []bool{true, true, false} {
		SetNodeUnschedulable(ctx, t, crClient, "node-0", unschedulable)

		node := &corev1.Node{}
		require.NoError(t, crClient.Get(ctx, crclient.ObjectKey{Name: "node-0"}, node))
		require.Equal(t, unschedulable, node.Spec.Unschedulable)
	}
}

func TestNodeSetPods(t *testing.T) {
	ctx := context.Background()
	nodeset := &slinkyv1beta1.NodeSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "slurm", Name: "slinky", UID: "nodeset-uid"},
	}
	owned := func(namespace, name string) *corev1.Pod {
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		pod.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: slinkyv1beta1.GroupVersion.String(),
			Kind:       "NodeSet",
			Name:       nodeset.Name,
			UID:        nodeset.UID,
			Controller: ptr.To(true),
		}}
		return pod
	}
	crClient := newClient(t,
		owned("slurm", "slinky-0"),
		owned("slurm", "slinky-1"),
		owned("other", "slinky-0"),
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "slurm", Name: "unowned"}},
	)

	pods, err := NodeSetPods(ctx, crClient, nodeset)
	require.NoError(t, err)
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}
	require.ElementsMatch(t, []string{"slinky-0", "slinky-1"}, names)
}

func TestPodDeadline(t *testing.T) {
	ctx := context.Background()
	deadline := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	crClient := newClient(t,
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace:   "slurm",
			Name:        "with-deadline",
			Annotations: map[string]string{slinkyv1beta1.AnnotationPodDeadline: deadline.Format(time.RFC3339)},
		}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Namespace:   "slurm",
			Name:        "bad-deadline",
			Annotations: map[string]string{slinkyv1beta1.AnnotationPodDeadline: "tomorrow"},
		}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "slurm", Name: "no-deadline"}},
	)

	tests := []struct {
		name    string
		pod     string
		want    time.Time
		wantErr bool
	}{
		{name: "annotated", pod: "with-deadline", want: deadline},
		{name: "unparsable annotation", pod: "bad-deadline", wantErr: true},
		{name: "no annotation", pod: "no-deadline", wantErr: true},
		{name: "missing pod", pod: "missing", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PodDeadline(ctx, crClient, crclient.ObjectKey{Namespace: "slurm", Name: tt.pod})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.True(t, tt.want.Equal(got), "got %s, want %s", got, tt.want)
		})
	}
}

func TestDefaultContainer(t *testing.T) {
	containers := []corev1.Container{{Name: "logfile"}, {Name: "slurmd"}}
	tests := []struct {
		name        string
		annotations map[string]string
		containers  []corev1.Container
		want        string
		wantErr     bool
	}{
		{name: "annotation", annotations: map[string]string{annotationDefaultContainer: "slurmd"}, containers: containers, want: "slurmd"},
		{name: "first container", containers: containers, want: "logfile"},
		{name: "no containers", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod", Annotations: tt.annotations},
				Spec:       corev1.PodSpec{Containers: tt.containers},
			}
			got, err := defaultContainer(pod)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
