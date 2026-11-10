// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accountingbuilder

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
	common "github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
)

func TestBuilder_BuildAccounting(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		accounting *slinkyv1beta1.Accounting
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
				accounting: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.AccountingSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildAccounting(tt.args.accounting)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildAccounting() error = %v, wantErr %v", err, tt.wantErr)
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

			case got.Spec.Template.Spec.Containers[0].Name != labels.AccountingApp:
				t.Errorf("Template.Spec.Containers[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Name, labels.AccountingApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].Name != labels.AccountingApp:
				t.Errorf("Template.Spec.Containers[0].Ports[0].Name = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, labels.AccountingApp)

			case got.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != common.SlurmdbdPort:
				t.Errorf("Template.Spec.Containers[0].Ports[0].ContainerPort = %v , want = %v",
					got.Spec.Template.Spec.Containers[0].Ports[0].Name, common.SlurmdbdPort)
			}
		})
	}
}

func TestBuilder_BuildAccounting_Env(t *testing.T) {
	accounting := &slinkyv1beta1.Accounting{
		ObjectMeta: metav1.ObjectMeta{Name: "slurm"},
		Spec: slinkyv1beta1.AccountingSpec{
			JwtKeyRef: &corev1.SecretKeySelector{},
			Slurmdbd: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Env: []corev1.EnvVar{{Name: "ABORT_ON_FATAL", Value: ""}, {Name: "SLURM_DEBUG_FLAGS", Value: "accrue"}},
				},
			},
		},
	}
	b := New(fake.NewFakeClient())
	got, err := b.BuildAccounting(accounting)
	require.NoError(t, err)
	require.Equal(t, labels.AccountingApp, got.Spec.Template.Spec.Containers[0].Name)
	require.ElementsMatch(t, accounting.Spec.Slurmdbd.Env, got.Spec.Template.Spec.Containers[0].Env)
}

func TestBuilder_BuildAccounting_TemplateMetadata(t *testing.T) {
	accounting := &slinkyv1beta1.Accounting{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "slurm",
			Annotations: map[string]string{"example.com/inherited": "true", "example.com/test": "false"},
			Labels:      map[string]string{"example.com/inherited": "true", "example.com/test": "false"},
		},
		Spec: slinkyv1beta1.AccountingSpec{
			JwtKeyRef: &corev1.SecretKeySelector{},
			Template: slinkyv1beta1.PodTemplate{
				Metadata: slinkyv1beta1.Metadata{
					Annotations: map[string]string{
						"example.com/test":         "true",
						"example.com/template":     "true",
						annotationDefaultContainer: "custom",
						annotationSlurmdbdConfHash: "custom",
					},
					Labels: map[string]string{
						"example.com/test":     "true",
						"example.com/template": "true",
						labels.AppLabel:        "custom",
					},
				},
			},
		},
	}
	b := New(fake.NewFakeClient())
	got, err := b.BuildAccounting(accounting)
	if err != nil {
		t.Fatalf("BuildAccounting(%q) error = %v", accounting.Name, err)
	}

	for key, want := range map[string]string{
		"example.com/inherited": "true",
		"example.com/test":      "true",
		"example.com/template":  "true",
		labels.AppLabel:         "slurmdbd",
	} {
		if value := got.Spec.Template.Labels[key]; value != want {
			t.Errorf("BuildAccounting(%q) pod label %q = %q, want %q", accounting.Name, key, value, want)
		}
	}
	for key, want := range map[string]string{
		"example.com/inherited":    "true",
		"example.com/test":         "true",
		"example.com/template":     "true",
		annotationDefaultContainer: "slurmdbd",
	} {
		if value := got.Spec.Template.Annotations[key]; value != want {
			t.Errorf("BuildAccounting(%q) pod annotation %q = %q, want %q", accounting.Name, key, value, want)
		}
	}
	if hash := got.Spec.Template.Annotations[annotationSlurmdbdConfHash]; hash == "custom" {
		t.Errorf("BuildAccounting(%q) pod config hash = %q, want an operator-generated hash", accounting.Name, hash)
	}
}

func BenchmarkBuilder_BuildAccounting(b *testing.B) {
	type fields struct {
		client client.Client
	}
	type args struct {
		accounting *slinkyv1beta1.Accounting
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
				accounting: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.AccountingSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
					},
				},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			client := New(bb.fields.client)

			for b.Loop() {
				_, err := client.BuildAccounting(bb.args.accounting)
				if err != nil {
					b.Fatalf("Failed to build accounting: %v", err)
				}
			}
		})
	}
}
