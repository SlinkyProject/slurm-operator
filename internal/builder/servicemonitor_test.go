// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder_test

import (
	"testing"

	"github.com/SlinkyProject/slurm-operator/internal/builder"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildServiceMonitor(t *testing.T) {
	tests := []struct {
		name    string
		c       client.Client
		opts    builder.ServiceMonitorOpts
		owner   metav1.Object
		want    *monitoringv1.ServiceMonitor
		wantErr bool
	}{
		{
			name:    "empty",
			c:       fake.NewFakeClient(),
			opts:    builder.ServiceMonitorOpts{},
			owner:   &corev1.Pod{},
			want:    &monitoringv1.ServiceMonitor{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := builder.New(tt.c)
			got, gotErr := b.BuildServiceMonitor(tt.opts, tt.owner)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("BuildServiceMonitor() failed: %v", gotErr)
				}
				return
			}
			switch {
			case tt.wantErr:
				t.Fatal("BuildServiceMonitor() succeeded unexpectedly")

			case !apiequality.Semantic.DeepEqual(got.Spec, tt.want.Spec):
				t.Errorf("BuildServiceMonitor() = %v, want %v", got, tt.want)
			}
		})
	}
}
