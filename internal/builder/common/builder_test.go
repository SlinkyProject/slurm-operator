// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
	"github.com/stretchr/testify/require"
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
		want *CommonBuilder
	}{
		{
			name: "nil",
			args: args{
				c: nil,
			},
			want: &CommonBuilder{
				client:      nil,
				refResolver: refresolver.New(nil),
			},
		},
		{
			name: "non-nil",
			args: args{
				c: c,
			},
			want: &CommonBuilder{
				client:      c,
				refResolver: refresolver.New(c),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, New(tt.args.c))
		})
	}
}
