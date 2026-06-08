// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"strings"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
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
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(got.Name, tt.args.name):
				t.Error("name does not match")
			}
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
			if !strings.Contains(got.Name, tt.args.name) {
				t.Error("name does not match")
			}
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
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(got.Name, tt.args.ref.Name):
				t.Error("name does not match")
			}
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
			if !strings.Contains(got.Name, tt.args.name) {
				t.Error("name does not match")
			}
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
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(got.Name, tt.args.ref.Name):
				t.Error("name does not match")
			}
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
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(got.Name, tt.args.name):
				t.Error("name does not match")
			}
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
			if !strings.Contains(got.Name, tt.args.name) {
				t.Error("name does not match")
			}
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
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(got.Name, tt.args.ref.Name):
				t.Error("name does not match")
			}
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
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(got.Name, tt.args.name):
				t.Error("name does not match")
			case ptr.Deref(got.Spec.Replicas, 0) != tt.args.replicas:
				t.Errorf("replicas do not match: got = %v, want = %v", ptr.Deref(got.Spec.Replicas, 0), tt.args.replicas)
			}
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
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(got.Name, tt.args.name):
				t.Error("name does not match")
			}
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
			if !strings.Contains(got.Name, tt.args.name) {
				t.Error("name does not match")
			}
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
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(got.Name, tt.args.ref.Name):
				t.Error("name does not match")
			}
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
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(got.Name, tt.args.name):
				t.Error("name does not match")
			}
		})
	}
}
