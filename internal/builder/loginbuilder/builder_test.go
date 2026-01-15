// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	"reflect"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	utilruntime.Must(slinkyv1beta1.AddToScheme(scheme.Scheme))
}

func TestNew(t *testing.T) {
	c := fake.NewFakeClient()
	type args struct {
		c client.Client
	}
	tests := []struct {
		name string
		args args
		want *LoginBuilder
	}{
		{
			name: "nil",
			args: args{
				c: nil,
			},
			want: &LoginBuilder{
				client:        nil,
				refResolver:   refresolver.New(nil),
				CommonBuilder: *common.New(nil),
			},
		},
		{
			name: "non-nil",
			args: args{
				c: c,
			},
			want: &LoginBuilder{
				client:        c,
				refResolver:   refresolver.New(c),
				CommonBuilder: *common.New(c),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.c); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}
func BenchmarkNew(b *testing.B) {
	c := fake.NewFakeClient()
	type args struct {
		c client.Client
	}
	benchmarks := []struct {
		name string
		args args
	}{
		{
			name: "nil",
			args: args{
				c: nil,
			},
		},
		{
			name: "non-nil",
			args: args{
				c: c,
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				New(bb.args.c)
			}
		})
	}
}
