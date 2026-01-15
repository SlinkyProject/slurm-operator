// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildContainer(t *testing.T) {
	tests := []struct {
		name   string
		client client.Client
		opts   ContainerOpts
		want   corev1.Container
	}{
		{
			name:   "empty",
			client: fake.NewFakeClient(),
			opts:   ContainerOpts{},
			want:   corev1.Container{},
		},
		{
			name:   "merge",
			client: fake.NewFakeClient(),
			opts: ContainerOpts{
				Base: corev1.Container{
					Name:            "foo",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args:            []string{"-a", "-b"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("500Mi"),
						},
					},
				},
				Merge: corev1.Container{
					Name:  "bar",
					Image: "nginx",
					Args:  []string{"-c"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				},
			},
			want: corev1.Container{
				Name:            "bar",
				Image:           "nginx",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args:            []string{"-a", "-b", "-c"},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.client)
			got := b.BuildContainer(tt.opts)
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("Builder.BuildContainer() = %v", got)
				return
			}
		})
	}
}

func BenchmarkBuilder_BuildContainer(b *testing.B) {
	benchmarks := []struct {
		name   string
		client client.Client
		opts   ContainerOpts
	}{
		{
			name:   "empty",
			client: fake.NewFakeClient(),
			opts:   ContainerOpts{},
		},
		{
			name:   "merge",
			client: fake.NewFakeClient(),
			opts: ContainerOpts{
				Base: corev1.Container{
					Name:            "foo",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args:            []string{"-a", "-b"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("500Mi"),
						},
					},
				},
				Merge: corev1.Container{
					Name:  "bar",
					Image: "nginx",
					Args:  []string{"-c"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			build := New(bb.client)

			for b.Loop() {
				build.BuildContainer(bb.opts)
			}
		})
	}
}
