// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	_ "embed"
	"testing"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
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
		loginset *slinkyv1alpha1.LoginSet
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
					WithObjects(&slinkyv1alpha1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					}).
					Build(),
			},
			args: args{
				loginset: &slinkyv1alpha1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1alpha1.LoginSetSpec{
						ControllerRef: slinkyv1alpha1.ObjectReference{
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
				loginset: &slinkyv1alpha1.LoginSet{
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

			case got.Spec.Template.Spec.Subdomain == "":
				t.Errorf("Template.Spec.Subdomain = %v , want = non-empty", got.Spec.Template.Spec.Subdomain)

			case got.Spec.Template.Spec.DNSConfig == nil:
				t.Errorf("Template.Spec.DNSConfig = %v , want = non-nil", got.Spec.Template.Spec.DNSConfig)

			case len(got.Spec.Template.Spec.DNSConfig.Searches) == 0:
				t.Errorf("len(Template.Spec.DNSConfig.Searches) = %v , want = > 0", len(got.Spec.Template.Spec.DNSConfig.Searches))
			}
		})
	}
}
