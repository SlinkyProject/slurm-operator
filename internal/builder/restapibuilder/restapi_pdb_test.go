// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapibuilder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	policyv1 "k8s.io/api/policy/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRestapiPodDisruptionBudgetName(t *testing.T) {
	ra := &slinkyv1beta1.RestApi{
		ObjectMeta: metav1.ObjectMeta{Name: "my-slurm", Namespace: "ns"},
	}
	if got, want := RestapiPodDisruptionBudgetName(ra), "my-slurm-restapi-pdb"; got != want {
		t.Fatalf("RestapiPodDisruptionBudgetName() = %q, want %q", got, want)
	}
}

func TestBuilder_BuildRestapiPodDisruptionBudget(t *testing.T) {
	b := New(fake.NewClientBuilder().Build())
	ra := &slinkyv1beta1.RestApi{
		ObjectMeta: metav1.ObjectMeta{Name: "slurm", Namespace: "default", UID: "uid"},
		Spec: slinkyv1beta1.RestApiSpec{
			ControllerRef: slinkyv1beta1.ObjectReference{Name: "slurm"},
		},
	}
	got, err := b.BuildRestapiPodDisruptionBudget(ra)
	if err != nil {
		t.Fatalf("BuildRestapiPodDisruptionBudget() error = %v", err)
	}
	want := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "slurm-restapi-pdb",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         slinkyv1beta1.RestApiAPIVersion,
					Kind:               slinkyv1beta1.RestApiKind,
					Name:               "slurm",
					UID:                "uid",
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels.NewBuilder().WithRestapiSelectorLabels(ra).Build(),
			},
			MinAvailable: ptr.To(intstr.FromInt32(1)),
		},
	}
	if !apiequality.Semantic.DeepEqual(got.Name, want.Name) || !apiequality.Semantic.DeepEqual(got.Namespace, want.Namespace) {
		t.Fatalf("metadata: got %#v, want name=%q ns=%q", got.ObjectMeta, want.Name, want.Namespace)
	}
	if !apiequality.Semantic.DeepEqual(got.Spec, want.Spec) {
		t.Errorf("Spec = %#v, want %#v", got.Spec, want.Spec)
	}
	if len(got.OwnerReferences) != 1 || got.OwnerReferences[0].Kind != slinkyv1beta1.RestApiKind {
		t.Errorf("OwnerReferences = %#v", got.OwnerReferences)
	}
}

func TestBuilder_BuildRestapiPodDisruptionBudget_customMinAvailable(t *testing.T) {
	b := New(fake.NewClientBuilder().Build())
	ra := &slinkyv1beta1.RestApi{
		ObjectMeta: metav1.ObjectMeta{Name: "slurm", Namespace: "default", UID: "uid"},
		Spec: slinkyv1beta1.RestApiSpec{
			ControllerRef: slinkyv1beta1.ObjectReference{Name: "slurm"},
			PodDisruptionBudget: &slinkyv1beta1.RestApiPodDisruptionBudget{
				PodDisruptionBudgetSpec: policyv1.PodDisruptionBudgetSpec{
					MinAvailable: ptr.To(intstr.FromInt32(2)),
				},
			},
		},
	}
	got, err := b.BuildRestapiPodDisruptionBudget(ra)
	if err != nil {
		t.Fatalf("BuildRestapiPodDisruptionBudget() error = %v", err)
	}
	if got.Spec.MinAvailable == nil || got.Spec.MinAvailable.IntValue() != 2 {
		t.Fatalf("MinAvailable = %#v, want 2", got.Spec.MinAvailable)
	}
}
