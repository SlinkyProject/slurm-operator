// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapibuilder

import (
	_ "embed"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildRestapi(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		restapi *slinkyv1beta1.RestApi
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					}).
					Build(),
			},
			args: args{
				restapi: &slinkyv1beta1.RestApi{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.RestApiSpec{
						ControllerRef: slinkyv1beta1.ObjectReference{
							Name: "slurm",
						},
					},
				},
			},
		},
		{
			name: "failure",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				restapi: &slinkyv1beta1.RestApi{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildRestapi(tt.args.restapi)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildRestapi() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case !set.KeySet(got.Spec.Template.Labels).HasAll(set.KeySet(got.Spec.Selector.MatchLabels).UnsortedList()...):
				t.Errorf("Template.Labels = %v , Selector.MatchLabels = %v",
					got.Spec.Template.Labels, got.Spec.Selector.MatchLabels)

			case ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot, false) != true:
				t.Errorf("got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot, true)

			case ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser, 0) != slurmrestdUserUid:
				t.Errorf("got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser, slurmrestdUserUid)

			case ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup, 0) != slurmrestdUserGid:
				t.Errorf("got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup, slurmrestdUserGid)

			case got.Spec.Template.Spec.Containers[0].Name != labels.RestapiApp:
				t.Errorf("Template.Spec.Containers[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Name, labels.RestapiApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].Name != labels.RestapiApp:
				t.Errorf("Template.Spec.Containers[0].Ports[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, labels.RestapiApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != SlurmrestdPort:
				t.Errorf("Template.Spec.Containers[0].Ports[0].ContainerPort = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, SlurmrestdPort)
			}
		})
	}
}

func BenchmarkBuilder_BuildRestapi(b *testing.B) {
	type fields struct {
		client client.Client
	}
	type args struct {
		restapi *slinkyv1beta1.RestApi
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					}).
					Build(),
			},
			args: args{
				restapi: &slinkyv1beta1.RestApi{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.RestApiSpec{
						ControllerRef: slinkyv1beta1.ObjectReference{
							Name: "slurm",
						},
					},
				},
			},
		},
		{
			name: "failure",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				restapi: &slinkyv1beta1.RestApi{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			build := New(bb.fields.client)

			for b.Loop() {
				build.BuildRestapi(bb.args.restapi) //nolint:errcheck
			}
		})
	}
}
