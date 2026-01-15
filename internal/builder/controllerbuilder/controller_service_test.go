// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildControllerService(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		controller *slinkyv1beta1.Controller
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *corev1.Service
		wantErr bool
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildControllerService(tt.args.controller)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildControllerService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got2, err := b.BuildController(tt.args.controller)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildController() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case !set.KeySet(got2.Labels).HasAll(set.KeySet(got.Spec.Selector).UnsortedList()...):
				t.Errorf("Labels = %v , Selector = %v", got.Labels, got.Spec.Selector)

			case got.Spec.Ports[0].TargetPort.String() != got2.Spec.Template.Spec.Containers[0].Ports[0].Name &&
				got.Spec.Ports[0].TargetPort.IntValue() != int(got2.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort):
				t.Errorf("Ports[0].TargetPort = %v , Template.Spec.Containers[0].Ports[0].Name = %v , Template.Spec.Containers[0].Ports[0].ContainerPort = %v",
					got.Spec.Ports[0].TargetPort,
					got2.Spec.Template.Spec.Containers[0].Ports[0].Name,
					got2.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
			}
		})
	}
}

func BenchmarkBuilder_BuildControllerService(b *testing.B) {
	type fields struct {
		client client.Client
	}
	type args struct {
		controller *slinkyv1beta1.Controller
	}
	benchmarks := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			client := New(bb.fields.client)
			_, err := client.BuildControllerService(bb.args.controller)
			if (err != nil) != bb.wantErr {
				b.Errorf("Builder.BuildControllerService() error = %v, wantErr %v", err, bb.wantErr)
				return
			}
			for b.Loop() {
				client.BuildController(bb.args.controller) //nolint:errcheck
			}
		})
	}
}
