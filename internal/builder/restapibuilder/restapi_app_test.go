// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapibuilder

import (
	_ "embed"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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
		name     string
		fields   fields
		args     args
		wantErr  bool
		wantArgs []string
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
						ControllerRef: corev1.LocalObjectReference{
							Name: "slurm",
						},
					},
				},
			},
			wantArgs: []string{"-s", "openapi/slurmctld", "0.0.0.0:6820"},
		},
		{
			name: "with accounting",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
						Spec: slinkyv1beta1.ControllerSpec{
							AccountingRef: &corev1.LocalObjectReference{
								Name: "slurm",
							},
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
						ControllerRef: corev1.LocalObjectReference{
							Name: "slurm",
						},
					},
				},
			},
			wantArgs: []string{"0.0.0.0:6820"},
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

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.True(t, set.KeySet(got.Spec.Template.Labels).HasAll(set.KeySet(got.Spec.Selector.MatchLabels).UnsortedList()...))
			require.True(t, ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot, false))
			require.Equal(t, slurmrestdUserUid, ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser, 0))
			require.Equal(t, slurmrestdUserGid, ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup, 0))
			require.Equal(t, labels.RestapiApp, got.Spec.Template.Spec.Containers[0].Name)
			require.Equal(t, labels.RestapiApp, got.Spec.Template.Spec.Containers[0].Ports[0].Name)
			require.Equal(t, int32(SlurmrestdPort), got.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
			require.Equal(t, tt.wantArgs, got.Spec.Template.Spec.Containers[0].Args)
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
						ControllerRef: corev1.LocalObjectReference{
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
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			client := New(bb.fields.client)
			for b.Loop() {
				_, err := client.BuildRestapi(bb.args.restapi)
				if (err != nil) != bb.wantErr {
					b.Errorf("Builder.BuildRestapi() error = %v, wantErr %v", err, bb.wantErr)
					return
				}
			}
		})
	}
}
