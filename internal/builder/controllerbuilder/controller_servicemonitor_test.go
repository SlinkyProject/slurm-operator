// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildControllerServiceMonitor(t *testing.T) {
	name := "slurm"
	slurmKeyRef := testutils.NewSlurmKeyRef(name)
	jwtKeyRef := testutils.NewJwtKeyRef(name)
	slurmKeySecret := testutils.NewSlurmKeySecret(slurmKeyRef)
	jwtKeySecret := testutils.NewJwtKeySecret(jwtKeyRef)
	controller := testutils.NewController(name, slurmKeyRef, jwtKeyRef, nil)
	controller.Spec.Metrics.Enabled = true
	controller.Spec.Metrics.ServiceMonitor.Enabled = true
	type testCase struct {
		name       string
		c          client.Client
		controller *slinkyv1beta1.Controller
		wantErr    bool
	}
	tests := []testCase{
		func() testCase {
			controller := controller.DeepCopy()
			fakeClient := fake.NewFakeClient(slurmKeySecret, jwtKeySecret, controller)
			return testCase{
				name:       "default endpoints",
				c:          fakeClient,
				controller: controller,
				wantErr:    false,
			}
		}(),
		func() testCase {
			controller := controller.DeepCopy()
			controller.Spec.Metrics.ServiceMonitor.MetricEndpoints = []slinkyv1beta1.MetricEndpoint{
				{
					Path:          "/metrics/nodes",
					Interval:      "30s",
					ScrapeTimeout: "25s",
				},
			}
			fakeClient := fake.NewFakeClient(slurmKeySecret, jwtKeySecret, controller)
			return testCase{
				name:       "custom endpoints",
				c:          fakeClient,
				controller: controller,
				wantErr:    false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.c)
			got, gotErr := b.BuildControllerServiceMonitor(tt.controller)

			if tt.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)

			got2, err := b.BuildController(tt.controller)

			require.NoError(t, err)
			require.True(t, set.KeySet(got2.Labels).HasAll(set.KeySet(got.Spec.Selector.MatchLabels).UnsortedList()...))
		})
	}
}
