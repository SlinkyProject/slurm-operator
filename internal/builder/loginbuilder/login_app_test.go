// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	_ "embed"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildLogin(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		loginset *slinkyv1beta1.LoginSet
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
				loginset: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.LoginSetSpec{
						ControllerRef: corev1.LocalObjectReference{
							Name: "slurm",
						},
					},
				},
			},
		},
		{
			name: "envars",
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
				loginset: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.LoginSetSpec{
						ControllerRef: corev1.LocalObjectReference{
							Name: "slurm",
						},
						Login: slinkyv1beta1.ContainerWrapper{
							Container: corev1.Container{
								Env: []corev1.EnvVar{
									{Name: "A", Value: "1"},
									{Name: "B", Value: "2"},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "custom ssh port",
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
				loginset: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.LoginSetSpec{
						ControllerRef: corev1.LocalObjectReference{
							Name: "slurm",
						},
						Login: slinkyv1beta1.ContainerWrapper{
							Container: corev1.Container{
								Ports: []corev1.ContainerPort{
									{
										Name:          labels.LoginApp,
										ContainerPort: 33,
										Protocol:      corev1.ProtocolTCP,
									},
								},
							},
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
				loginset: &slinkyv1beta1.LoginSet{
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
			got, err := b.BuildLogin(tt.args.loginset)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.True(t, set.KeySet(got.Spec.Template.Labels).HasAll(set.KeySet(got.Spec.Selector.MatchLabels).UnsortedList()...))
			require.Equal(t, labels.LoginApp, got.Spec.Template.Spec.Containers[0].Name)
			require.Equal(t, labels.LoginApp, got.Spec.Template.Spec.Containers[0].Ports[0].Name)

			if len(tt.args.loginset.Spec.Login.Ports) > 0 && tt.args.loginset.Spec.Login.Ports[0].ContainerPort != 0 {
				require.Equal(t, tt.args.loginset.Spec.Login.Ports[0].ContainerPort, got.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
			} else {
				require.Equal(t, LoginPort, got.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
			}

			require.NotNil(t, got.Spec.Template.Spec.DNSConfig)
			require.NotEmpty(t, got.Spec.Template.Spec.DNSConfig.Searches)

			if tt.name == "envars" {
				envs := got.Spec.Template.Spec.Containers[0].Env
				envMap := make(map[string]struct{})

				for _, env := range envs {
					_, exists := envMap[env.Name]
					require.False(t, exists, "duplicate env var: %s", env.Name)
					envMap[env.Name] = struct{}{}
				}

				_, hasA := envMap["A"]
				require.True(t, hasA, "env var A not found")

				_, hasB := envMap["B"]
				require.True(t, hasB, "env var B not found")

				_, hasSackd := envMap["SACKD_OPTIONS"]
				require.True(t, hasSackd, "env var SACKD_OPTIONS not found")
			}
		})
	}
}

func BenchmarkBuilder_BuildLogin(b *testing.B) {
	type fields struct {
		client client.Client
	}
	type args struct {
		loginset *slinkyv1beta1.LoginSet
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
				loginset: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.LoginSetSpec{
						ControllerRef: corev1.LocalObjectReference{
							Name: "slurm",
						},
					},
				},
			},
		},
		{
			name: "envars",
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
				loginset: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.LoginSetSpec{
						ControllerRef: corev1.LocalObjectReference{
							Name: "slurm",
						},
						Login: slinkyv1beta1.ContainerWrapper{
							Container: corev1.Container{
								Env: []corev1.EnvVar{
									{Name: "A", Value: "1"},
									{Name: "B", Value: "2"},
								},
							},
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
				loginset: &slinkyv1beta1.LoginSet{
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
				_, err := client.BuildLogin(bb.args.loginset)
				if (err != nil) != bb.wantErr {
					b.Errorf("Failed to build login %v", err)
					return
				}
			}
		})
	}
}
