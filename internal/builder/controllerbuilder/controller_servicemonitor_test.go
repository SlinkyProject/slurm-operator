// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildControllerServiceMonitor(t *testing.T) {
	name := "slurm"
	slurmKeyRef := testutils.NewSlurmKeyRef(name)
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
	slurmKeySecret := testutils.NewSlurmKeySecret(slurmKeyRef)
	jwtHs256KeySecret := testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
	controller := testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
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
			fakeClient := fake.NewFakeClient(slurmKeySecret, jwtHs256KeySecret, controller)
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
			fakeClient := fake.NewFakeClient(slurmKeySecret, jwtHs256KeySecret, controller)
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
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("BuildControllerServiceMonitor() failed: %v", gotErr)
				}
				return
			}
			got2, err := b.BuildController(tt.controller)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildControllerServiceMonitor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case tt.wantErr:
				t.Fatal("BuildControllerServiceMonitor() succeeded unexpectedly")

			case !set.KeySet(got2.Labels).HasAll(set.KeySet(got.Spec.Selector.MatchLabels).UnsortedList()...):
				t.Errorf("Labels = %v , Selector = %v", got.Labels, got.Spec.Selector)
			}
		})
	}
}

func BenchmarkBuilder_BuildControllerServiceMonitor(b *testing.B) {
	name := "slurm"
	slurmKeyRef := testutils.NewSlurmKeyRef(name)
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
	slurmKeySecret := testutils.NewSlurmKeySecret(slurmKeyRef)
	jwtHs256KeySecret := testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
	controller := testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
	controller.Spec.Metrics.Enabled = true
	controller.Spec.Metrics.ServiceMonitor.Enabled = true
	type testCase struct {
		name       string
		c          client.Client
		controller *slinkyv1beta1.Controller
		wantErr    bool
	}
	benchmarks := []testCase{
		func() testCase {
			controller := controller.DeepCopy()
			fakeClient := fake.NewFakeClient(slurmKeySecret, jwtHs256KeySecret, controller)
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
			fakeClient := fake.NewFakeClient(slurmKeySecret, jwtHs256KeySecret, controller)
			return testCase{
				name:       "custom endpoints",
				c:          fakeClient,
				controller: controller,
				wantErr:    false,
			}
		}(),
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			build := New(bb.c)
			_, gotErr := build.BuildControllerServiceMonitor(bb.controller)
			if gotErr != nil {
				if !bb.wantErr {
					b.Errorf("BuildControllerServiceMonitor() failed: %v", gotErr)
				}
				return
			}
			for b.Loop() {
				build.BuildController(bb.controller) //nolint:errcheck
			}
		})
	}
}
