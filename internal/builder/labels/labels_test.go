// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package labels

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewBuilder(t *testing.T) {
	type args struct {
		builder *Builder
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Empty",
			args: args{
				builder: NewBuilder(),
			},
			want: map[string]string{},
		},
		{
			name: "WithApp",
			args: args{
				builder: NewBuilder().
					WithApp("foo"),
			},
			want: map[string]string{
				AppLabel: "foo",
			},
		},
		{
			name: "WithComponent",
			args: args{
				builder: NewBuilder().
					WithComponent("foo"),
			},
			want: map[string]string{
				componentLabel: "foo",
			},
		},
		{
			name: "WithInstance",
			args: args{
				builder: NewBuilder().
					WithInstance("foo"),
			},
			want: map[string]string{
				instanceLabel: "foo",
			},
		},
		{
			name: "WithManagedBy",
			args: args{
				builder: NewBuilder().
					WithManagedBy("foo"),
			},
			want: map[string]string{
				managedbyLabel: "foo",
			},
		},
		{
			name: "WithPartOf",
			args: args{
				builder: NewBuilder().
					WithPartOf("foo"),
			},
			want: map[string]string{
				partOfLabel: "foo",
			},
		},
		{
			name: "WithCluster",
			args: args{
				builder: NewBuilder().
					WithCluster("slurm"),
			},
			want: map[string]string{
				clusterLabel: "slurm",
			},
		},
		{
			name: "WithPodProtect",
			args: args{
				builder: NewBuilder().
					WithPodProtect(),
			},
			want: map[string]string{
				slinkyv1beta1.LabelNodeSetPodProtect: "true",
			},
		},
		{
			name: "WithLabels",
			args: args{
				builder: NewBuilder().
					WithLabels(map[string]string{
						"foo": "bar",
					}),
			},
			want: map[string]string{
				"foo": "bar",
			},
		},
		{
			name: "WithControllerSelectorLabels",
			args: args{
				builder: NewBuilder().
					WithControllerSelectorLabels(
						&slinkyv1beta1.Controller{
							ObjectMeta: v1.ObjectMeta{
								Name: "test",
							},
						},
					),
			},
			want: map[string]string{
				instanceLabel: "test",
				AppLabel:      ControllerApp,
			},
		},
		{
			name: "WithControllerLabels",
			args: args{
				builder: NewBuilder().
					WithControllerLabels(
						&slinkyv1beta1.Controller{
							ObjectMeta: v1.ObjectMeta{
								Name: "test",
							},
						},
					),
			},
			want: map[string]string{
				instanceLabel:  "test",
				AppLabel:       ControllerApp,
				componentLabel: ControllerComp,
			},
		},
		{
			name: "WithRestapiSelectorLabels",
			args: args{
				builder: NewBuilder().
					WithRestapiSelectorLabels(
						&slinkyv1beta1.RestApi{
							ObjectMeta: v1.ObjectMeta{
								Name: "test",
							},
						},
					),
			},
			want: map[string]string{
				instanceLabel: "test",
				AppLabel:      RestapiApp,
			},
		},
		{
			name: "WithRestapiLabels",
			args: args{
				builder: NewBuilder().
					WithRestapiLabels(
						&slinkyv1beta1.RestApi{
							ObjectMeta: v1.ObjectMeta{
								Name: "test",
							},
						},
					),
			},
			want: map[string]string{
				instanceLabel:  "test",
				AppLabel:       RestapiApp,
				componentLabel: RestapiComp,
			},
		},
		{
			name: "WithAccountingSelectorLabels",
			args: args{
				builder: NewBuilder().
					WithAccountingSelectorLabels(
						&slinkyv1beta1.Accounting{
							ObjectMeta: v1.ObjectMeta{
								Name: "test",
							},
						},
					),
			},
			want: map[string]string{
				instanceLabel: "test",
				AppLabel:      AccountingApp,
			},
		},
		{
			name: "WithAccountingLabels",
			args: args{
				builder: NewBuilder().
					WithAccountingLabels(
						&slinkyv1beta1.Accounting{
							ObjectMeta: v1.ObjectMeta{
								Name: "test",
							},
						},
					),
			},
			want: map[string]string{
				instanceLabel:  "test",
				AppLabel:       AccountingApp,
				componentLabel: AccountingComp,
			},
		},
		{
			name: "WithWorkerSelectorLabels",
			args: args{
				builder: NewBuilder().
					WithWorkerSelectorLabels(
						&slinkyv1beta1.NodeSet{
							ObjectMeta: v1.ObjectMeta{
								Name: "test",
							},
						},
					),
			},
			want: map[string]string{
				instanceLabel: "test",
				AppLabel:      WorkerApp,
			},
		},
		{
			name: "WithWorkerLabels",
			args: args{
				builder: NewBuilder().
					WithWorkerLabels(
						&slinkyv1beta1.NodeSet{
							ObjectMeta: v1.ObjectMeta{
								Name: "test",
							},
							Spec: slinkyv1beta1.NodeSetSpec{
								ControllerRef: slinkyv1beta1.ObjectReference{
									Name: "slurm",
								},
							},
						},
					),
			},
			want: map[string]string{
				instanceLabel:  "test",
				AppLabel:       WorkerApp,
				componentLabel: WorkerComp,
				clusterLabel:   "slurm",
			},
		},
		{
			name: "WithLoginSelectorLabels",
			args: args{
				builder: NewBuilder().
					WithLoginSelectorLabels(
						&slinkyv1beta1.LoginSet{
							ObjectMeta: v1.ObjectMeta{
								Name: "test",
							},
						},
					),
			},
			want: map[string]string{
				instanceLabel: "test",
				AppLabel:      LoginApp,
			},
		},
		{
			name: "WithLoginLabels",
			args: args{
				builder: NewBuilder().
					WithLoginLabels(
						&slinkyv1beta1.LoginSet{
							ObjectMeta: v1.ObjectMeta{
								Name: "test",
							},
						},
					),
			},
			want: map[string]string{
				instanceLabel:  "test",
				AppLabel:       LoginApp,
				componentLabel: LoginComp,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.args.builder.Build()
			if !equality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkNewBuilder(b *testing.B) {
	type args struct {
		builder *Builder
	}
	benchmarks := []struct {
		name string
		args args
	}{
		{
			name: "Empty",
			args: args{
				builder: NewBuilder(),
			},
		},
		{
			name: "WithApp",
			args: args{
				builder: NewBuilder().
					WithApp("foo"),
			},
		},
		{
			name: "WithComponent",
			args: args{
				builder: NewBuilder().
					WithComponent("foo"),
			},
		},
		{
			name: "WithInstance",
			args: args{
				builder: NewBuilder().
					WithInstance("foo"),
			},
		},
		{
			name: "WithManagedBy",
			args: args{
				builder: NewBuilder().
					WithManagedBy("foo"),
			},
		},
		{
			name: "WithPartOf",
			args: args{
				builder: NewBuilder().
					WithPartOf("foo"),
			},
		},
		{
			name: "WithCluster",
			args: args{
				builder: NewBuilder().
					WithCluster("slurm"),
			},
		},
		{
			name: "WithPodProtect",
			args: args{
				builder: NewBuilder().
					WithPodProtect(),
			},
		},
		{
			name: "WithLabels",
			args: args{
				builder: NewBuilder().
					WithLabels(map[string]string{
						"foo": "bar",
					}),
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				bb.args.builder.Build()
			}
		})
	}
}
