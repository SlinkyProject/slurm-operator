// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildRestapiService(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		restapi *slinkyv1beta1.RestApi
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *corev1.Service
		wantErr bool
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					}).
					Build(),
			},
			args: args{
				restapi: &slinkyv1beta1.RestApi{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.RestApiSpec{
						ControllerRef: slinkyv1beta1.ObjectReference{
							Name: "slurm",
						},
					},
				},
			},
		},
		{
			name: "with external IPs",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					}).
					Build(),
			},
			args: args{
				restapi: &slinkyv1beta1.RestApi{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.RestApiSpec{
						ControllerRef: slinkyv1beta1.ObjectReference{
							Name: "slurm",
						},
						Service: slinkyv1beta1.ServiceSpec{
							ServiceSpecWrapper: slinkyv1beta1.ServiceSpecWrapper{
								ServiceSpec: corev1.ServiceSpec{
									ExternalIPs: []string{"169.254.169.254"},
								},
							},
						},
					},
				},
			},
			want: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "slurmrestd",
							Protocol:   "TCP",
							Port:       6820,
							TargetPort: intstr.FromString("slurmrestd"),
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "slurm",
						"app.kubernetes.io/name":     "slurmrestd",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildRestapiService(tt.args.restapi)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildRestapiService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got2, err := b.BuildRestapi(tt.args.restapi)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildRestapi() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case !set.KeySet(got2.Labels).HasAll(set.KeySet(got.Spec.Selector).UnsortedList()...):
				t.Errorf("Labels = %v , Selector = %v", got.Labels, got.Spec.Selector)

			case got.Spec.Ports[0].TargetPort.String() != got2.Spec.Template.Spec.Containers[0].Ports[0].Name &&
				got.Spec.Ports[0].TargetPort.IntValue() != int(got2.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort):
				t.Errorf("Ports[0].TargetPort = %v , Template.Spec.Containers[0].Ports[0].Name = %v , Template.Spec.Containers[0].Ports[0].ContainerPort = %v",
					got.Spec.Ports[0].TargetPort,
					got2.Spec.Template.Spec.Containers[0].Ports[0].Name,
					got2.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
			}
			if tt.want != nil {
				if !apiequality.Semantic.DeepEqual(tt.want.Spec, got.Spec) {
					t.Errorf("Wanted service = %v, Got service = %v", tt.want.Spec, got.Spec)
				}
			}
		})
	}
}
