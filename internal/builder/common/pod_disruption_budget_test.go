// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/stretchr/testify/require"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildPodDisruptionBudget(t *testing.T) {
	tests := []struct {
		name    string
		c       client.Client
		opts    PodDisruptionBudgetOpts
		owner   metav1.Object
		want    *policyv1.PodDisruptionBudget
		wantErr bool
	}{
		{
			name: "Empty",
			c:    fake.NewFakeClient(),
			opts: PodDisruptionBudgetOpts{},
			owner: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "slurm-worker-slinky",
				},
			},
			want: &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{},
					Annotations: map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "slinky.slurm.net/v1beta1",
							Kind:               "NodeSet",
							Name:               "slurm-worker-slinky",
							UID:                "",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "No owner",
			c:       fake.NewFakeClient(),
			opts:    PodDisruptionBudgetOpts{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.c)
			got, gotErr := b.BuildPodDisruptionBudget(tt.opts, tt.owner)

			if tt.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tt.want, got)
		})
	}
}
