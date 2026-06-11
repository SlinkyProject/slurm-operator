// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildServiceMonitor(t *testing.T) {
	tests := []struct {
		name    string
		c       client.Client
		opts    ServiceMonitorOpts
		owner   metav1.Object
		want    *monitoringv1.ServiceMonitor
		wantErr bool
	}{
		{
			name:  "empty",
			c:     fake.NewFakeClient(),
			opts:  ServiceMonitorOpts{},
			owner: &corev1.Pod{},
			want: &monitoringv1.ServiceMonitor{
				Spec: monitoringv1.ServiceMonitorSpec{
					TargetLabels:    []string{},
					PodTargetLabels: []string{},
					Endpoints:       []monitoringv1.Endpoint{},
					ScrapeProtocols: []monitoringv1.ScrapeProtocol{},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.c)
			got, gotErr := b.BuildServiceMonitor(tt.opts, tt.owner)

			if tt.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tt.want.Spec, got.Spec)
		})
	}
}
