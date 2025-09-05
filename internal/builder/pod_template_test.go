// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_buildPodTemplate(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		opts PodTemplateOpts
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   corev1.PodTemplateSpec
	}{
		{
			name: "empty",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				opts: PodTemplateOpts{},
			},
			want: corev1.PodTemplateSpec{},
		},
		{
			name: "containers",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				opts: PodTemplateOpts{
					base: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: "foo"},
						},
						Containers: []corev1.Container{
							{Name: "foo"},
						},
					},
					merge: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: "bar"},
						},
						Containers: []corev1.Container{
							{Name: "bar"},
						},
					},
				},
			},
			want: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{Name: "foo"},
						{Name: "bar"},
					},
					Containers: []corev1.Container{
						{Name: "foo"},
						{Name: "bar"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			if got := b.buildPodTemplate(tt.args.opts); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("Builder.buildPodTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}
