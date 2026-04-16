// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func Test_restapiPodDisruptionBudgetDesired(t *testing.T) {
	tests := []struct {
		name    string
		restapi *slinkyv1beta1.RestApi
		want    bool
	}{
		{
			name: "single replica",
			restapi: &slinkyv1beta1.RestApi{
				Spec: slinkyv1beta1.RestApiSpec{Replicas: ptr.To[int32](1)},
			},
			want: false,
		},
		{
			name: "two replicas default",
			restapi: &slinkyv1beta1.RestApi{
				Spec: slinkyv1beta1.RestApiSpec{Replicas: ptr.To[int32](2)},
			},
			want: true,
		},
		{
			name: "two replicas explicit enabled",
			restapi: &slinkyv1beta1.RestApi{
				Spec: slinkyv1beta1.RestApiSpec{
					Replicas: ptr.To[int32](2),
					PodDisruptionBudget: &slinkyv1beta1.RestApiPodDisruptionBudget{
						Enabled: ptr.To(true),
					},
				},
			},
			want: true,
		},
		{
			name: "two replicas disabled",
			restapi: &slinkyv1beta1.RestApi{
				Spec: slinkyv1beta1.RestApiSpec{
					Replicas: ptr.To[int32](2),
					PodDisruptionBudget: &slinkyv1beta1.RestApiPodDisruptionBudget{
						Enabled: ptr.To(false),
					},
				},
			},
			want: false,
		},
		{
			name: "nil replicas defaults to one",
			restapi: &slinkyv1beta1.RestApi{
				ObjectMeta: metav1.ObjectMeta{Name: "x"},
				Spec:       slinkyv1beta1.RestApiSpec{},
			},
			want: false,
		},
		{
			name: "five replicas without pdb block",
			restapi: &slinkyv1beta1.RestApi{
				Spec: slinkyv1beta1.RestApiSpec{Replicas: ptr.To[int32](5)},
			},
			want: true,
		},
		{
			name: "three replicas with minAvailable only in embedded PDB spec",
			restapi: &slinkyv1beta1.RestApi{
				Spec: slinkyv1beta1.RestApiSpec{
					Replicas: ptr.To[int32](3),
					PodDisruptionBudget: &slinkyv1beta1.RestApiPodDisruptionBudget{
						PodDisruptionBudgetSpec: policyv1.PodDisruptionBudgetSpec{
							MinAvailable: ptr.To(intstr.FromInt(2)),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "two replicas with maxUnavailable percentage in embedded PDB spec",
			restapi: &slinkyv1beta1.RestApi{
				Spec: slinkyv1beta1.RestApiSpec{
					Replicas: ptr.To[int32](2),
					PodDisruptionBudget: &slinkyv1beta1.RestApiPodDisruptionBudget{
						PodDisruptionBudgetSpec: policyv1.PodDisruptionBudgetSpec{
							MaxUnavailable: ptr.To(intstr.FromString("25%")),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "two replicas with unhealthyPodEvictionPolicy in embedded PDB spec",
			restapi: &slinkyv1beta1.RestApi{
				Spec: slinkyv1beta1.RestApiSpec{
					Replicas: ptr.To[int32](2),
					PodDisruptionBudget: &slinkyv1beta1.RestApiPodDisruptionBudget{
						PodDisruptionBudgetSpec: policyv1.PodDisruptionBudgetSpec{
							MinAvailable:               ptr.To(intstr.FromInt(1)),
							UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "two replicas disabled even when embedded spec has minAvailable",
			restapi: &slinkyv1beta1.RestApi{
				Spec: slinkyv1beta1.RestApiSpec{
					Replicas: ptr.To[int32](2),
					PodDisruptionBudget: &slinkyv1beta1.RestApiPodDisruptionBudget{
						Enabled: ptr.To(false),
						PodDisruptionBudgetSpec: policyv1.PodDisruptionBudgetSpec{
							MinAvailable: ptr.To(intstr.FromInt(1)),
						},
					},
				},
			},
			want: false,
		},
		{
			name: "zero replicas",
			restapi: &slinkyv1beta1.RestApi{
				Spec: slinkyv1beta1.RestApiSpec{Replicas: ptr.To[int32](0)},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := restapiPodDisruptionBudgetDesired(tt.restapi); got != tt.want {
				t.Fatalf("restapiPodDisruptionBudgetDesired() = %v, want %v", got, tt.want)
			}
		})
	}
}
