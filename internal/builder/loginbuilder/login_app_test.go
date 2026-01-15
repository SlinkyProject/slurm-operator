// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	_ "embed"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
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
						ControllerRef: slinkyv1beta1.ObjectReference{
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
						ControllerRef: slinkyv1beta1.ObjectReference{
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildLogin(tt.args.loginset)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildLogin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case !set.KeySet(got.Spec.Template.Labels).HasAll(set.KeySet(got.Spec.Selector.MatchLabels).UnsortedList()...):
				t.Errorf("Template.Labels = %v , Selector.MatchLabels = %v",
					got.Spec.Template.Labels, got.Spec.Selector.MatchLabels)

			case got.Spec.Template.Spec.Containers[0].Name != labels.LoginApp:
				t.Errorf("Template.Spec.Containers[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Name, labels.LoginApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].Name != labels.LoginApp:
				t.Errorf("Template.Spec.Containers[0].Ports[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, labels.LoginApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != LoginPort:
				t.Errorf("Template.Spec.Containers[0].Ports[0].ContainerPort = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, LoginPort)

			case got.Spec.Template.Spec.DNSConfig == nil:
				t.Errorf("Template.Spec.DNSConfig = %v , want = non-nil", got.Spec.Template.Spec.DNSConfig)

			case len(got.Spec.Template.Spec.DNSConfig.Searches) == 0:
				t.Errorf("len(Template.Spec.DNSConfig.Searches) = %v , want = > 0", len(got.Spec.Template.Spec.DNSConfig.Searches))
			}
			if tt.name == "envars" {
				envs := got.Spec.Template.Spec.Containers[0].Env
				envMap := make(map[string]struct{})
				for _, env := range envs {
					if _, exists := envMap[env.Name]; exists {
						t.Errorf("duplicate env var: %s", env.Name)
					}
					envMap[env.Name] = struct{}{}
				}
				if _, ok := envMap["A"]; !ok {
					t.Errorf("env var A not found")
				}
				if _, ok := envMap["B"]; !ok {
					t.Errorf("env var B not found")
				}
				if _, ok := envMap["SACKD_OPTIONS"]; !ok {
					t.Errorf("env var SACKD_OPTIONS not found")
				}
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
			build := New(bb.fields.client)

			for b.Loop() {
				build.BuildLogin(bb.args.loginset) //nolint:errcheck
			}
		})
	}
}
