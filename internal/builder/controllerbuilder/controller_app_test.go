// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	_ "embed"
	"reflect"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildController() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case !set.KeySet(got.Spec.Template.Labels).HasAll(set.KeySet(got.Spec.Selector.MatchLabels).UnsortedList()...):
				t.Errorf("Template.Labels = %v , Selector.MatchLabels = %v",
					got.Spec.Template.Labels, got.Spec.Selector.MatchLabels)

			case ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot, false) != true:
				t.Errorf("got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot, true)

			case ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser, 0) != common.SlurmUserUid:
				t.Errorf("got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser, common.SlurmUserUid)

			case ptr.Deref(got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup, 0) != common.SlurmUserGid:
				t.Errorf("got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].SecurityContext.RunAsGroup, common.SlurmUserGid)

			case got.Spec.Template.Spec.Containers[0].Name != labels.ControllerApp:
				t.Errorf("Template.Spec.Containers[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Name, labels.ControllerApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].Name != labels.ControllerApp:
				t.Errorf("Template.Spec.Containers[0].Ports[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, labels.ControllerApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != common.SlurmctldPort:
				t.Errorf("Template.Spec.Containers[0].Ports[0].ContainerPort = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, common.SlurmctldPort)
			}
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

func TestBuildController_Singleton(t *testing.T) {
	c := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{Name: "slurm", Namespace: "slurm"},
	}
	b := New(fake.NewFakeClient())
	sts, err := b.BuildController(c)
	if err != nil {
		t.Fatalf("BuildController() error = %v", err)
	}
	if got := ptr.Deref(sts.Spec.Replicas, -1); got != 1 {
		t.Errorf("Replicas = %d, want 1", got)
	}
	if sts.Spec.ServiceName != "slurm-controller" {
		t.Errorf("ServiceName = %q, want slurm-controller", sts.Spec.ServiceName)
	}
	if got := sts.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path; got != common.SlurmReadyz {
		t.Errorf("singleton readiness path = %q, want %q", got, common.SlurmReadyz)
	}
	if aff := sts.Spec.Template.Spec.Affinity; aff != nil && aff.PodAntiAffinity != nil &&
		len(aff.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		t.Error("singleton should not have required pod anti-affinity")
	}
}

func TestBuildController_HARequiresExistingClaim(t *testing.T) {
	c := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{Name: "slurm", Namespace: "slurm"},
		Spec: slinkyv1beta1.ControllerSpec{
			Replicas: ptr.To(int32(2)),
			Persistence: slinkyv1beta1.ControllerPersistence{
				Enabled: ptr.To(true),
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
			Replicas: ptr.To(int32(2)),
			Persistence: slinkyv1beta1.ControllerPersistence{
				Enabled:       ptr.To(true),
				ExistingClaim: "slurm-statesave",
			},
		},
	}
	b := New(fake.NewFakeClient())
	sts, err := b.BuildController(c)
	if err != nil {
		t.Fatalf("BuildController() error = %v", err)
	}
	if got := ptr.Deref(sts.Spec.Replicas, -1); got != 2 {
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
	wantSelector := labels.NewBuilder().WithControllerSelectorLabels(c).Build()
	if term.LabelSelector == nil || !reflect.DeepEqual(term.LabelSelector.MatchLabels, wantSelector) {
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
