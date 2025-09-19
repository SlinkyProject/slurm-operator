// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package labels

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
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
				appLabel: "foo",
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
			name: "WithController",
			args: args{
				builder: NewBuilder().
					WithController("slurm"),
			},
			want: map[string]string{
				controllerLabel: "slurm",
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
