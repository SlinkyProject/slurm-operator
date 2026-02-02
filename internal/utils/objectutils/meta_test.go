// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestKeyFunc(t *testing.T) {
	type args struct {
		obj metav1.Object
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "No namespace",
			args: args{
				obj: &appsv1.Deployment{},
			},
			want: "/",
		},
		{
			name: "Slurm namespace",
			args: args{
				obj: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
					},
				},
			},
			want: "bar/foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KeyFunc(tt.args.obj); got != tt.want {
				t.Errorf("KeyFunc() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNamespacedName(t *testing.T) {
	type args struct {
		obj metav1.Object
	}
	tests := []struct {
		name string
		args args
		want types.NamespacedName
	}{
		{
			name: "No namespace",
			args: args{
				obj: &appsv1.Deployment{},
			},
			want: types.NamespacedName{},
		},
		{
			name: "Slurm namespace",
			args: args{
				obj: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
					},
				},
			},
			want: types.NamespacedName{
				Name:      "foo",
				Namespace: "bar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NamespacedName(tt.args.obj); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("NamespacedName() = %v, want %v", got, tt.want)
			}
		})
	}
}
