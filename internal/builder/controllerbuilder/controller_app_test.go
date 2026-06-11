// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	_ "embed"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildController(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		controller *slinkyv1beta1.Controller
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
					},
				},
			},
		},
		{
			name: "with persistence",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						Persistence: slinkyv1beta1.ControllerPersistence{
							Enabled: ptr.To(true),
						},
						JwtKeyRef: &corev1.SecretKeySelector{},
					},
				},
			},
		},
		{
			name: "with persistence from claim",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						Persistence: slinkyv1beta1.ControllerPersistence{
							Enabled:       ptr.To(true),
							ExistingClaim: "pvc",
						},
						JwtKeyRef: &corev1.SecretKeySelector{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildController(tt.args.controller)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.True(t, set.KeySet(got.Spec.Template.Labels).HasAll(set.KeySet(got.Spec.Selector.MatchLabels).UnsortedList()...))
			require.True(t, ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot, false))
			require.Equal(t, common.SlurmUserUid, ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser, 0))
			require.Equal(t, common.SlurmUserGid, ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup, 0))
			require.Equal(t, labels.ControllerApp, got.Spec.Template.Spec.Containers[0].Name)
			require.Equal(t, labels.ControllerApp, got.Spec.Template.Spec.Containers[0].Ports[0].Name)
			require.Equal(t, int32(common.SlurmctldPort), got.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
		})
	}
}

func BenchmarkBuilder_BuildController(b *testing.B) {
	type fields struct {
		client client.Client
	}
	type args struct {
		controller *slinkyv1beta1.Controller
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
					},
				},
			},
		},
		{
			name: "with persistence",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						Persistence: slinkyv1beta1.ControllerPersistence{
							Enabled: ptr.To(true),
						},
						JwtKeyRef: &corev1.SecretKeySelector{},
					},
				},
			},
		},
		{
			name: "with persistence from claim",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						Persistence: slinkyv1beta1.ControllerPersistence{
							Enabled:       ptr.To(true),
							ExistingClaim: "pvc",
						},
						JwtKeyRef: &corev1.SecretKeySelector{},
					},
				},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			build := New(bb.fields.client)

			for b.Loop() {
				build.BuildController(bb.args.controller) //nolint:errcheck
			}
		})
	}
}
