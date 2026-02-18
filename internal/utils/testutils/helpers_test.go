// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"reflect"
	"strings"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNewObjectRef(t *testing.T) {
	type args struct {
		obj client.Object
	}
	tests := []struct {
		name string
		args args
		want slinkyv1beta1.ObjectReference
	}{
		{
			name: "empty",
			args: args{
				obj: &corev1.Pod{},
			},
			want: slinkyv1beta1.ObjectReference{},
		},
		{
			name: "named",
			args: args{
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: corev1.NamespaceDefault,
					},
				},
			},
			want: slinkyv1beta1.ObjectReference{
				Namespace: corev1.NamespaceDefault,
				Name:      "foo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewObjectRef(tt.args.obj); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewObjectRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewController(t *testing.T) {
	type args struct {
		name           string
		slurmKeyRef    corev1.SecretKeySelector
		jwtHs256KeyRef corev1.SecretKeySelector
		accounting     *slinkyv1beta1.Accounting
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "without accounting",
			args: args{
				name:           "foo",
				slurmKeyRef:    NewSlurmKeyRef("foo"),
				jwtHs256KeyRef: NewJwtHs256KeyRef("foo"),
				accounting:     nil,
			},
		},
		{
			name: "with accounting",
			args: args{
				name:           "foo",
				slurmKeyRef:    NewSlurmKeyRef("foo"),
				jwtHs256KeyRef: NewJwtHs256KeyRef("foo"),
				accounting:     NewAccounting("foo", NewSlurmKeyRef("foo"), NewJwtHs256KeyRef("foo"), NewPasswordRef("name")),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewController(tt.args.name, tt.args.slurmKeyRef, tt.args.jwtHs256KeyRef, tt.args.accounting)
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(NewObjectRef(got).Name, tt.args.name):
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
			case !strings.Contains(NewObjectRef(got).Name, tt.args.ref.Name):
				t.Error("name does not match")
			}
		})
	}
}

func TestNewJwtHs256KeyRef(t *testing.T) {
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
			got := NewJwtHs256KeyRef(tt.args.name)
			if !strings.Contains(got.Name, tt.args.name) {
				t.Error("name does not match")
			}
		})
	}
}

func TestNewJwtHs256KeySecret(t *testing.T) {
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
				ref: NewJwtHs256KeyRef("foo"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewJwtHs256KeySecret(tt.args.ref)
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(NewObjectRef(got).Name, tt.args.ref.Name):
				t.Error("name does not match")
			}
		})
	}
}

func TestNewAccounting(t *testing.T) {
	type args struct {
		name           string
		slurmKeyRef    corev1.SecretKeySelector
		jwtHs256KeyRef corev1.SecretKeySelector
		passwordRef    corev1.SecretKeySelector
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "smoke",
			args: args{
				name:           "foo",
				slurmKeyRef:    NewSlurmKeyRef("foo"),
				jwtHs256KeyRef: NewJwtHs256KeyRef("foo"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAccounting(tt.args.name, tt.args.slurmKeyRef, tt.args.jwtHs256KeyRef, tt.args.passwordRef)
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(NewObjectRef(got).Name, tt.args.name):
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
			case !strings.Contains(NewObjectRef(got).Name, tt.args.ref.Name):
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
				controller: NewController("foo", NewSlurmKeyRef("foo"), NewJwtHs256KeyRef("foo"), nil),
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
			case !strings.Contains(NewObjectRef(got).Name, tt.args.name):
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
				controller:  NewController("foo", NewSlurmKeyRef("foo"), NewJwtHs256KeyRef("foo"), nil),
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
			case !strings.Contains(NewObjectRef(got).Name, tt.args.name):
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
			case !strings.Contains(NewObjectRef(got).Name, tt.args.ref.Name):
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
				controller: NewController("foo", NewSlurmKeyRef("foo"), NewJwtHs256KeyRef("foo"), nil),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewRestapi(tt.args.name, tt.args.controller)
			switch {
			case got == nil:
				t.Error("returned object was nil")
			case !strings.Contains(NewObjectRef(got).Name, tt.args.name):
				t.Error("name does not match")
			}
		})
	}
}
