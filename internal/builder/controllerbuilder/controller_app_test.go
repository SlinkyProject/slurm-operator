// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
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

func TestBuildController_HARequiresExistingClaim(t *testing.T) {
	c := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{Name: "slurm", Namespace: "slurm"},
		Spec: slinkyv1beta1.ControllerSpec{
			HighAvailability: slinkyv1beta1.ControllerHighAvailability{
				Enabled: true,
				Backups: new(int32(2)),
			},
			Persistence: slinkyv1beta1.ControllerPersistence{
				Enabled: new(true),
				// no ExistingClaim -> must error
			},
		},
	}
	b := New(fake.NewFakeClient())
	if _, err := b.BuildController(c); err == nil {
		t.Fatal("BuildController() with replicas>1 and no existingClaim: want error, got nil")
	}
}

func TestBuildController_HA(t *testing.T) {
	c := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{Name: "slurm", Namespace: "slurm"},
		Spec: slinkyv1beta1.ControllerSpec{
			HighAvailability: slinkyv1beta1.ControllerHighAvailability{
				Enabled: true,
				Backups: new(int32(2)),
			},
			Persistence: slinkyv1beta1.ControllerPersistence{
				Enabled:       new(true),
				ExistingClaim: "slurm-statesave",
			},
		},
	}
	b := New(fake.NewFakeClient())
	sts, err := b.BuildController(c)
	if err != nil {
		t.Fatalf("BuildController() error = %v", err)
	}
	if got := ptr.Deref(sts.Spec.Replicas, -1); got != c.Replicas() {
		t.Errorf("Replicas = %d, want 2", got)
	}
	aff := sts.Spec.Template.Spec.Affinity
	if aff == nil || aff.PodAntiAffinity == nil ||
		len(aff.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) == 0 {
		t.Fatal("HA should set required pod anti-affinity")
	}
	term := aff.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0]
	if term.TopologyKey != corev1.LabelHostname {
		t.Errorf("anti-affinity topologyKey = %q, want %q", term.TopologyKey, corev1.LabelHostname)
	}
	wantSelector := labels.NewBuilder().
		WithControllerSelectorLabels(c).
		Build()
	if term.LabelSelector == nil || !apiequality.Semantic.DeepEqual(term.LabelSelector.MatchLabels, wantSelector) {
		t.Errorf("anti-affinity selector = %v, want %v", term.LabelSelector, wantSelector)
	}
	if len(sts.Spec.VolumeClaimTemplates) != 0 {
		t.Errorf("HA must not use volumeClaimTemplates, got %d", len(sts.Spec.VolumeClaimTemplates))
	}
	// A standby never answers /readyz with 2xx, so /readyz readiness would
	// leave it permanently NotReady and wedge StatefulSet rolling updates.
	if got := sts.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path; got != common.SlurmLivez {
		t.Errorf("HA readiness path = %q, want %q", got, common.SlurmLivez)
	}
}
