// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"testing"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildClusterWideWorkerService(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		nodeset *slinkyv1alpha1.NodeSet
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
				nodeset: &slinkyv1alpha1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gpu-1",
						Namespace: "slinky",
					},
					Spec: slinkyv1alpha1.NodeSetSpec{
						ControllerRef: slinkyv1alpha1.ObjectReference{
							Name: "slurm",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildClusterWideWorkerService(tt.args.nodeset)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildClusterWideWorkerService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case got.Name != slurmClusterWorkerServiceName(tt.args.nodeset.Spec.ControllerRef.Name):
				t.Errorf("Service.Name = %v, want %v", got.Name, slurmClusterWorkerServiceName(tt.args.nodeset.Spec.ControllerRef.Name))

			case got.Spec.ClusterIP != corev1.ClusterIPNone:
				t.Errorf("Service.Spec.ClusterIP = %v, want headless service", got.Spec.ClusterIP)

			case len(got.Spec.Ports) != 1 || got.Spec.Ports[0].Port != SlurmdPort:
				t.Errorf("Service.Spec.Ports = %v, want single port %d", got.Spec.Ports, SlurmdPort)
			}
		})
	}
}
