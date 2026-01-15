// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accountingbuilder

import (
	_ "embed"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	common "github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildAccounting(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		accounting *slinkyv1beta1.Accounting
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
				accounting: &slinkyv1beta1.Accounting{
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
			got, err := b.BuildAccounting(tt.args.accounting)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildAccounting() error = %v, wantErr %v", err, tt.wantErr)
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

			case ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser, 0) != common.SlurmUserUid:
				t.Errorf("got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser, common.SlurmUserUid)

			case ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup, 0) != common.SlurmUserGid:
				t.Errorf("got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup, common.SlurmUserGid)

			case got.Spec.Template.Spec.Containers[0].Name != labels.AccountingApp:
				t.Errorf("Template.Spec.Containers[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Name, labels.AccountingApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].Name != labels.AccountingApp:
				t.Errorf("Template.Spec.Containers[0].Ports[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, labels.AccountingApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != common.SlurmdbdPort:
				t.Errorf("Template.Spec.Containers[0].Ports[0].ContainerPort = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, common.SlurmdbdPort)
			}
		})
	}
}

func BenchmarkBuilder_BuildAccounting(b *testing.B) {
	type fields struct {
		client client.Client
	}
	type args struct {
		accounting *slinkyv1beta1.Accounting
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				accounting: &slinkyv1beta1.Accounting{
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
				build.BuildAccounting(bb.args.accounting) //nolint:errcheck
			}
		})
	}
}
