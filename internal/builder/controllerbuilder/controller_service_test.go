// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildControllerService(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		controller *slinkyv1beta1.Controller
	}
	defaultPorts := []corev1.ServicePort{
		{
			Name:       "slurmctld",
			Protocol:   "TCP",
			Port:       6817,
			TargetPort: intstr.FromString("slurmctld"),
		},
	}
	defaultSelector := map[string]string{
		"app.kubernetes.io/instance": "slurm",
		"app.kubernetes.io/name":     "slurmctld",
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *corev1.Service
	}{
		{
			// Singleton (default): pre-HA Service, not headless.
			name: "default",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
					},
				},
			},
			want: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports:    defaultPorts,
					Selector: defaultSelector,
				},
			},
		},
		{
			// Singleton: user nodePort override is honored, as pre-HA.
			name: "with nodeport",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
						Service: slinkyv1beta1.ServiceSpec{
							NodePort: 32500,
						},
					},
				},
			},
			want: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "slurmctld",
							Protocol:   "TCP",
							Port:       6817,
							TargetPort: intstr.FromString("slurmctld"),
							NodePort:   32500,
						},
					},
					Selector: defaultSelector,
				},
			},
		},
		{
			// Singleton: user spec wrapper is applied, but BuildService strips
			// externalIPs (CVE-2020-8554).
			name: "with external IPs",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
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
					Ports:    defaultPorts,
					Selector: defaultSelector,
				},
			},
		},
		{
			// Singleton: ServiceSpecWrapper is applied to the built Service.
			name: "with service spec wrapper",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
						Service: slinkyv1beta1.ServiceSpec{
							ServiceSpecWrapper: slinkyv1beta1.ServiceSpecWrapper{
								ServiceSpec: corev1.ServiceSpec{
									Type:            corev1.ServiceTypeNodePort,
									SessionAffinity: corev1.ServiceAffinityClientIP,
								},
							},
						},
					},
				},
			},
			want: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Type:            corev1.ServiceTypeNodePort,
					SessionAffinity: corev1.ServiceAffinityClientIP,
					Ports:           defaultPorts,
					Selector:        defaultSelector,
				},
			},
		},
		{
			// Singleton: Port override is applied to the built Service.
			name: "with port override",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
						Service: slinkyv1beta1.ServiceSpec{
							Port: 16817,
						},
					},
				},
			},
			want: &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "slurmctld",
							Protocol:   "TCP",
							Port:       16817,
							TargetPort: intstr.FromString("slurmctld"),
							NodePort:   0,
						},
					},
					Selector: defaultSelector,
				},
			},
		},
		{
			// HA: the governing Service is headless with per-pod DNS and
			// publishNotReadyAddresses, on the fixed slurmctld port.
			name: "ha governing service is headless",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
						Replicas:  ptr.To[int32](2),
						Persistence: slinkyv1beta1.ControllerPersistence{
							Enabled:       ptr.To(true),
							ExistingClaim: "slurm-statesave",
						},
					},
				},
			},
			want: &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP:                corev1.ClusterIPNone,
					Ports:                    defaultPorts,
					Selector:                 defaultSelector,
					PublishNotReadyAddresses: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildControllerService(tt.args.controller)
			if err != nil {
				t.Fatalf("Builder.BuildControllerService() error = %v", err)
			}
			if got.Name != "slurm-controller" {
				t.Errorf("name = %q, want slurm-controller", got.Name)
			}
			got2, err := b.BuildController(tt.args.controller)
			if err != nil {
				t.Fatalf("Builder.BuildController() error = %v", err)
			}
			switch {
			case !set.KeySet(got2.Labels).HasAll(set.KeySet(got.Spec.Selector).UnsortedList()...):
				t.Errorf("Labels = %v , Selector = %v", got.Labels, got.Spec.Selector)

			case got.Spec.Ports[0].TargetPort.String() != got2.Spec.Template.Spec.Containers[0].Ports[0].Name &&
				got.Spec.Ports[0].TargetPort.IntValue() != int(got2.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort):
				t.Errorf("Ports[0].TargetPort = %v , Template.Spec.Containers[0].Ports[0].Name = %v , Template.Spec.Containers[0].Ports[0].ContainerPort = %v",
					got.Spec.Ports[0].TargetPort,
					got2.Spec.Template.Spec.Containers[0].Ports[0].Name,
					got2.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
			}
			if !apiequality.Semantic.DeepEqual(tt.want.Spec, got.Spec) {
				t.Errorf("Wanted service = %v, Got service = %v", tt.want.Spec, got.Spec)
			}
		})
	}
}

// The governing Service must stay headless under HA, so user Service
// overrides are rejected rather than silently ignored. port/nodePort are also
// rejected by CEL on ControllerSpec; the spec wrapper is schema-less
// (preserve-unknown-fields), so the builder is its only enforcement point.
func TestBuilder_BuildControllerService_HARejectsOverrides(t *testing.T) {
	tests := []struct {
		name    string
		service slinkyv1beta1.ServiceSpec
	}{
		{
			name:    "nodePort",
			service: slinkyv1beta1.ServiceSpec{NodePort: 32500},
		},
		{
			name:    "port",
			service: slinkyv1beta1.ServiceSpec{Port: 16817},
		},
		{
			name: "spec wrapper",
			service: slinkyv1beta1.ServiceSpec{
				ServiceSpecWrapper: slinkyv1beta1.ServiceSpecWrapper{
					ServiceSpec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &slinkyv1beta1.Controller{
				ObjectMeta: metav1.ObjectMeta{Name: "slurm", Namespace: "slurm"},
				Spec: slinkyv1beta1.ControllerSpec{
					JwtKeyRef: &corev1.SecretKeySelector{},
					Replicas:  ptr.To[int32](2),
					Service:   tt.service,
				},
			}
			b := New(fake.NewFakeClient())
			if _, err := b.BuildControllerService(controller); err == nil {
				t.Error("BuildControllerService() error = nil, want error for service override with replicas > 1")
			}
		})
	}
}
