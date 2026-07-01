// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package indexes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func Test_getPodNodeName(t *testing.T) {
	tests := []struct {
		name string
		o    client.Object
		want []string
	}{
		{
			name: "Pod",
			o:    &corev1.Pod{},
			want: []string{""},
		},
		{
			name: "Pod, with nodeName",
			o: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeName: "foo",
				},
			},
			want: []string{"foo"},
		},
		{
			name: "invalid",
			o:    &corev1.Node{},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPodNodeName(tt.o)
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("getPodNodeName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewFakeClientBuilderWithIndexes(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "smoke",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewFakeClientBuilderWithIndexes()
			if got == nil {
				t.Errorf("NewFakeClientBuilderWithIndexes() = %v", got)
			}
		})
	}
}

func TestNodeSetConfigRefsIndex(t *testing.T) {
	ctx := context.Background()

	opted := &slinkyv1beta1.NodeSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "opted",
			Namespace:   corev1.NamespaceDefault,
			Annotations: map[string]string{slinkyv1beta1.AnnotationReloadOnChange: "true"},
		},
	}
	opted.Spec.Template.PodSpecWrapper.PodSpec.Volumes = []corev1.Volume{
		{Name: "v", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "tracked-cm"}}}},
	}

	notOpted := &slinkyv1beta1.NodeSet{
		ObjectMeta: metav1.ObjectMeta{Name: "plain", Namespace: corev1.NamespaceDefault},
	}
	notOpted.Spec.Template.PodSpecWrapper.PodSpec.Volumes = opted.Spec.Template.PodSpecWrapper.PodSpec.Volumes

	c := NewFakeClientBuilderWithIndexes().WithObjects(opted, notOpted).Build()

	list := &slinkyv1beta1.NodeSetList{}
	if err := c.List(ctx, list,
		client.InNamespace(corev1.NamespaceDefault),
		client.MatchingFields{NodeSetConfigRefsField: "configmap/tracked-cm"}); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].Name != "opted" {
		t.Fatalf("expected only the opted-in NodeSet, got %d items: %+v", len(list.Items), list.Items)
	}
}
