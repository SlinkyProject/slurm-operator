// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	normSS := func(m map[string]string) map[string]string {
		if m == nil {
			return map[string]string{}
		}
		return m
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(fake.NewFakeClient())
			got, err := b.BuildService(tt.args.opts, tt.args.owner)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.args.opts.Key.String(), objectutils.KeyFunc(got))
			require.Equal(t, normSS(tt.args.opts.Metadata.Annotations), got.Annotations)
			require.Equal(t, normSS(tt.args.opts.Metadata.Labels), got.Labels)
			require.True(t, set.KeySet(got.Spec.Selector).HasAll(set.KeySet(tt.args.opts.Selector).UnsortedList()...))

			if tt.args.opts.Headless {
				require.Equal(t, corev1.ClusterIPNone, got.Spec.ClusterIP)
			}
		})
	}
}
