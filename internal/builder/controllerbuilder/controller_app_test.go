// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
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

func TestBuilder_BuildController_configHash(t *testing.T) {
	primary := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "slurm-config", Namespace: "ns"},
		Data:       map[string]string{"slurm.conf": "ClusterName=foo"},
	}
	extra := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "extra-conf", Namespace: "ns"},
		Data:       map[string]string{"cgroup.conf": "ConstrainCores=yes"},
	}
	extraChanged := extra.DeepCopy()
	extraChanged.Data["cgroup.conf"] = "ConstrainCores=no"
	controller := func(inplace bool, refs ...string) *slinkyv1beta1.Controller {
		c := &slinkyv1beta1.Controller{
			ObjectMeta: metav1.ObjectMeta{Name: "slurm", Namespace: "ns"},
			Spec: slinkyv1beta1.ControllerSpec{
				JwtKeyRef:          &corev1.SecretKeySelector{},
				InplaceReconfigure: inplace,
			},
		}
		for _, name := range refs {
			c.Spec.ConfigFileRefs = append(c.Spec.ConfigFileRefs, corev1.LocalObjectReference{Name: name})
		}
		return c
	}

	type fields struct {
		client client.Client
	}
	type args struct {
		controller *slinkyv1beta1.Controller
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantHash string
	}{
		{
			name: "inplace skips config hash",
			fields: fields{
				client: fake.NewFakeClient(primary, extra),
			},
			args: args{
				controller: controller(true, "extra-conf"),
			},
		},
		{
			name: "primary config only",
			fields: fields{
				client: fake.NewFakeClient(primary),
			},
			args: args{
				controller: controller(false),
			},
			wantHash: crypto.CheckSumFromMap(primary.Data),
		},
		{
			name: "includes ConfigFileRefs",
			fields: fields{
				client: fake.NewFakeClient(primary, extra),
			},
			args: args{
				controller: controller(false, "extra-conf"),
			},
			wantHash: crypto.CheckSumFromMap(map[string]string{
				"slurm.conf":  "ClusterName=foo",
				"cgroup.conf": "ConstrainCores=yes",
			}),
		},
		{
			name: "ConfigFileRefs change hash",
			fields: fields{
				client: fake.NewFakeClient(primary, extraChanged),
			},
			args: args{
				controller: controller(false, "extra-conf"),
			},
			wantHash: crypto.CheckSumFromMap(map[string]string{
				"slurm.conf":  "ClusterName=foo",
				"cgroup.conf": "ConstrainCores=no",
			}),
		},
		{
			name: "missing ConfigFileRefs ConfigMap",
			fields: fields{
				client: fake.NewFakeClient(primary),
			},
			args: args{
				controller: controller(false, "extra-conf"),
			},
			wantHash: crypto.CheckSumFromMap(primary.Data),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sts, err := New(tt.fields.client).BuildController(tt.args.controller)
			require.NoError(t, err)
			require.Equal(t, tt.wantHash, sts.Spec.Template.Annotations[annotationSlurmConfigHash])
		})
	}
}
