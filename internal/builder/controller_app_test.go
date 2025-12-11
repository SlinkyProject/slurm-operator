// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

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

func TestBuilder_BuildController(t *testing.T) {
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
		{
			name: "with persistence",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						Persistence: slinkyv1beta1.ControllerPersistence{
							Enabled: true,
						},
					},
				},
			},
		},
		{
			name: "with persistence from claim",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						Persistence: slinkyv1beta1.ControllerPersistence{
							Enabled:       true,
							ExistingClaim: "pvc",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildController(tt.args.controller)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildController() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case !set.KeySet(got.Spec.Template.Labels).HasAll(set.KeySet(got.Spec.Selector.MatchLabels).UnsortedList()...):
				t.Errorf("Template.Labels = %v , Selector.MatchLabels = %v",
					got.Spec.Template.Labels, got.Spec.Selector.MatchLabels)

			case got.Spec.Template.Spec.Containers[0].SecurityContext != nil:
				t.Errorf("got.Spec.Template.Spec.Containers[0].SecurityContext = %v , want = nil",
					got.Spec.Template.Spec.Containers[0].SecurityContext)

			case ptr.Deref(got.Spec.Template.Spec.SecurityContext.FSGroup, 0) != slurmUserGid:
				t.Errorf("got.Spec.Template.Spec.SecurityContext.FSGroup = %v , want = %v",
					got.Spec.Template.Spec.SecurityContext.FSGroup, slurmUserGid)

			case got.Spec.Template.Spec.Containers[0].Name != labels.ControllerApp:
				t.Errorf("Template.Spec.Containers[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Name, labels.ControllerApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].Name != labels.ControllerApp:
				t.Errorf("Template.Spec.Containers[0].Ports[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, labels.ControllerApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != SlurmctldPort:
				t.Errorf("Template.Spec.Containers[0].Ports[0].ContainerPort = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, SlurmctldPort)
			}
		})
	}
}
