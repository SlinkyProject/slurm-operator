// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestNewController(t *testing.T) {
	type args struct {
		name        string
		slurmKeyRef corev1.SecretKeySelector
		jwtKeyRef   corev1.SecretKeySelector
		accounting  *slinkyv1beta1.Accounting
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "without accounting",
			args: args{
				name:        "foo",
				slurmKeyRef: NewSlurmKeyRef("foo"),
				jwtKeyRef:   NewJwtKeyRef("foo"),
				accounting:  nil,
			},
		},
		{
			name: "with accounting",
			args: args{
				name:        "foo",
				slurmKeyRef: NewSlurmKeyRef("foo"),
				jwtKeyRef:   NewJwtKeyRef("foo"),
				accounting:  NewAccounting("foo", NewSlurmKeyRef("foo"), NewJwtKeyRef("foo"), NewPasswordRef("name")),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewController(tt.args.name, tt.args.slurmKeyRef, tt.args.jwtKeyRef, tt.args.accounting)

			require.NotNil(t, got)
			require.Contains(t, got.Name, tt.args.name)
		})
	}
}

func TestNewSlurmKeyRef(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				name: "foo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSlurmKeyRef(tt.args.name)
			require.Contains(t, got.Name, tt.args.name)
		})
	}
}

func TestNewSlurmKeySecret(t *testing.T) {
	type args struct {
		ref corev1.SecretKeySelector
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				ref: NewSlurmKeyRef("foo"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSlurmKeySecret(tt.args.ref)

			require.NotNil(t, got)
			require.Contains(t, got.Name, tt.args.ref.Name)
		})
	}
}

func TestNewJwtKeyRef(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				name: "foo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewJwtKeyRef(tt.args.name)

			require.Contains(t, got.Name, tt.args.name)
		})
	}
}

func TestNewJwtKeySecret(t *testing.T) {
	type args struct {
		ref corev1.SecretKeySelector
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				ref: NewJwtKeyRef("foo"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewJwtKeySecret(tt.args.ref)

			require.NotNil(t, got)
			require.Contains(t, got.Name, tt.args.ref.Name)
		})
	}
}

func TestNewAccounting(t *testing.T) {
	type args struct {
		name        string
		slurmKeyRef corev1.SecretKeySelector
		jwtKeyRef   corev1.SecretKeySelector
		passwordRef corev1.SecretKeySelector
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				name:        "foo",
				slurmKeyRef: NewSlurmKeyRef("foo"),
				jwtKeyRef:   NewJwtKeyRef("foo"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAccounting(tt.args.name, tt.args.slurmKeyRef, tt.args.jwtKeyRef, tt.args.passwordRef)

			require.NotNil(t, got)
			require.Contains(t, got.Name, tt.args.name)
		})
	}
}

func TestNewPasswordRef(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				name: "foo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPasswordRef(tt.args.name)

			require.Contains(t, got.Name, tt.args.name)
		})
	}
}

func TestNewPasswordSecret(t *testing.T) {
	type args struct {
		ref corev1.SecretKeySelector
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				ref: NewPasswordRef("foo"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPasswordSecret(tt.args.ref)

			require.NotNil(t, got)
			require.Contains(t, got.Name, tt.args.ref.Name)
		})
	}
}

func TestNewNodeset(t *testing.T) {
	type args struct {
		name       string
		controller *slinkyv1beta1.Controller
		replicas   int32
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				name:       "foo",
				controller: NewController("foo", NewSlurmKeyRef("foo"), NewJwtKeyRef("foo"), nil),
				replicas:   2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewNodeset(tt.args.name, tt.args.controller, tt.args.replicas)

			require.NotNil(t, got)
			require.Contains(t, got.Name, tt.args.name)
			require.Equal(t, tt.args.replicas, ptr.Deref(got.Spec.Replicas, 0))
		})
	}
}

func TestNewLoginset(t *testing.T) {
	type args struct {
		name        string
		controller  *slinkyv1beta1.Controller
		sssdConfRef corev1.SecretKeySelector
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				name:        "foo",
				controller:  NewController("foo", NewSlurmKeyRef("foo"), NewJwtKeyRef("foo"), nil),
				sssdConfRef: NewSssdConfRef("foo"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewLoginset(tt.args.name, tt.args.controller, tt.args.sssdConfRef)

			require.NotNil(t, got)
			require.Contains(t, got.Name, tt.args.name)
		})
	}
}

func TestNewSssdConfRef(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				name: "foo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSssdConfRef(tt.args.name)

			require.Contains(t, got.Name, tt.args.name)
		})
	}
}

func TestNewSssdConfSecret(t *testing.T) {
	type args struct {
		ref corev1.SecretKeySelector
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				ref: NewSssdConfRef("foo"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSssdConfSecret(tt.args.ref)

			require.NotNil(t, got)
			require.Contains(t, got.Name, tt.args.ref.Name)
		})
	}
}

func TestNewRestapi(t *testing.T) {
	type args struct {
		name       string
		controller *slinkyv1beta1.Controller
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				name:       "foo",
				controller: NewController("foo", NewSlurmKeyRef("foo"), NewJwtKeyRef("foo"), nil),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewRestapi(tt.args.name, tt.args.controller)

			require.NotNil(t, got)
			require.Contains(t, got.Name, tt.args.name)
		})
	}
}
