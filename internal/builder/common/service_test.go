// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildService(t *testing.T) {
	type args struct {
		opts  ServiceOpts
		owner metav1.Object
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "empty",
			args: args{
				owner: &appsv1.Deployment{},
			},
		},
		{
			name:    "bad owner",
			wantErr: true,
		},
		{
			name: "with options",
			args: args{
				opts: ServiceOpts{
					Key: types.NamespacedName{
						Name:      "foo",
						Namespace: "bar",
					},
					Metadata: slinkyv1beta1.Metadata{
						Annotations: map[string]string{
							"foo": "bar",
						},
						Labels: map[string]string{
							"fizz": "buzz",
						},
					},
					Selector: map[string]string{
						"fizz": "buzz",
					},
					ServiceSpec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Name: "foo", Port: 0},
							{Name: "bar", Port: 1},
						},
					},
					Headless: true,
				},
				owner: &appsv1.Deployment{},
			},
		},
		{
			name: "duplicate port name",
			args: args{
				opts: ServiceOpts{
					ServiceSpec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Name: "foo", Port: 0},
							{Name: "foo", Port: 1},
						},
					},
				},
				owner: &appsv1.Deployment{},
			},
			wantErr: true,
		},
		{
			name: "duplicate port number",
			args: args{
				opts: ServiceOpts{
					ServiceSpec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Name: "foo", Port: 0},
							{Name: "bar", Port: 0},
						},
					},
				},
				owner: &appsv1.Deployment{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(fake.NewFakeClient())
			got, err := b.BuildService(tt.args.opts, tt.args.owner)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case objectutils.KeyFunc(got) != tt.args.opts.Key.String():
				t.Errorf("NamespacedName = %v , want = %v", objectutils.KeyFunc(got), tt.args.opts.Key.String())

			case !apiequality.Semantic.DeepEqual(got.Annotations, tt.args.opts.Metadata.Annotations):
				t.Errorf("Annotations = %v , want = %v", got.Annotations, tt.args.opts.Metadata.Annotations)

			case !apiequality.Semantic.DeepEqual(got.Labels, tt.args.opts.Metadata.Labels):
				t.Errorf("Labels = %v , want = %v", got.Labels, tt.args.opts.Metadata.Labels)

			case !set.KeySet(got.Spec.Selector).HasAll(set.KeySet(tt.args.opts.Selector).UnsortedList()...):
				t.Errorf("Selector = %v , want = %v", got.Spec.Selector, tt.args.opts.Selector)

			case tt.args.opts.Headless && got.Spec.ClusterIP != corev1.ClusterIPNone:
				t.Errorf("Headless enabled but `ClusterIP != %s`", corev1.ClusterIPNone)
			}
		})
	}
}

func BenchmarkBuilder_BuildService(b *testing.B) {
	type args struct {
		opts  ServiceOpts
		owner metav1.Object
	}
	benchmarks := []struct {
		name string
		args args
	}{
		{
			name: "empty",
			args: args{
				owner: &appsv1.Deployment{},
			},
		},
		{
			name: "bad owner",
		},
		{
			name: "with options",
			args: args{
				opts: ServiceOpts{
					Key: types.NamespacedName{
						Name:      "foo",
						Namespace: "bar",
					},
					Metadata: slinkyv1beta1.Metadata{
						Annotations: map[string]string{
							"foo": "bar",
						},
						Labels: map[string]string{
							"fizz": "buzz",
						},
					},
					Selector: map[string]string{
						"fizz": "buzz",
					},
					ServiceSpec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Name: "foo", Port: 0},
							{Name: "bar", Port: 1},
						},
					},
					Headless: true,
				},
				owner: &appsv1.Deployment{},
			},
		},
		{
			name: "duplicate port name",
			args: args{
				opts: ServiceOpts{
					ServiceSpec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Name: "foo", Port: 0},
							{Name: "foo", Port: 1},
						},
					},
				},
				owner: &appsv1.Deployment{},
			},
		},
		{
			name: "duplicate port number",
			args: args{
				opts: ServiceOpts{
					ServiceSpec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Name: "foo", Port: 0},
							{Name: "bar", Port: 0},
						},
					},
				},
				owner: &appsv1.Deployment{},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			build := New(fake.NewFakeClient())

			for b.Loop() {
				build.BuildService(bb.args.opts, bb.args.owner) //nolint:errcheck
			}
		})
	}
}
