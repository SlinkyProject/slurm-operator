// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package workerbuilder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	policyv1 "k8s.io/api/policy/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildClusterWorkerPodDisruptionBudget(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		c       client.Client
		nodeset *slinkyv1beta1.NodeSet
		want    *policyv1.PodDisruptionBudget
		wantErr bool
	}{
		{
			name:    "Empty",
			c:       fake.NewFakeClient(),
			nodeset: &slinkyv1beta1.NodeSet{},
			want: &policyv1.PodDisruptionBudget{
				ObjectMeta: v1.ObjectMeta{
					Name: "slurm-workers-pdb-",
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					Selector: &v1.LabelSelector{
						MatchLabels: labels.NewBuilder().WithPodProtect().WithCluster("").Build(),
					},
					MaxUnavailable: ptr.To(intstr.FromInt(0)),
				},
			},
			wantErr: false,
		},
		{
			name: "With Owned Nodeset",
			c:    fake.NewFakeClient(),
			nodeset: &slinkyv1beta1.NodeSet{
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: slinkyv1beta1.ObjectReference{
						Name: "slurm",
					},
				},
			},
			want: &policyv1.PodDisruptionBudget{
				ObjectMeta: v1.ObjectMeta{
					Name: "slurm-workers-pdb-slurm",
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					Selector: &v1.LabelSelector{
						MatchLabels: labels.NewBuilder().WithPodProtect().WithCluster("slurm").Build(),
					},
					MaxUnavailable: ptr.To(intstr.FromInt(0)),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.c)
			got, gotErr := b.BuildClusterWorkerPodDisruptionBudget(tt.nodeset)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("BuildClusterWorkerPodDisruptionBudget() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("BuildClusterWorkerPodDisruptionBudget() succeeded unexpectedly")
			}
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("BuildClusterWorkerPodDisruptionBudget() = %v, want %v", got, tt.want)
			}
		})
	}
}
