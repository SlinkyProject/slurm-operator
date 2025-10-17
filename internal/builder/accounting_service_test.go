// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"testing"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildAccountingService(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		accounting *slinkyv1alpha1.Accounting
	}
	tests := []struct {
		name                         string
		fields                       fields
		args                         args
		wantErr                      bool
		wantClusterIP                string
		wantPublishNotReadyAddresses bool
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "mariadb",
						},
						Data: map[string][]byte{
							"password": []byte("mariadb-password"),
						},
					}).
					Build(),
			},
			args: args{
				accounting: &slinkyv1alpha1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1alpha1.AccountingSpec{
						StorageConfig: slinkyv1alpha1.StorageConfig{
							PasswordKeyRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb",
								},
								Key: "password",
							},
						},
					},
				},
			},
			wantClusterIP:                "",
			wantPublishNotReadyAddresses: false,
		},
		{
			name: "headless service",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "mariadb",
						},
						Data: map[string][]byte{
							"password": []byte("mariadb-password"),
						},
					}).
					Build(),
			},
			args: args{
				accounting: &slinkyv1alpha1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1alpha1.AccountingSpec{
						Service: slinkyv1alpha1.ServiceSpec{
							ServiceSpecWrapper: slinkyv1alpha1.ServiceSpecWrapper{
								ServiceSpec: corev1.ServiceSpec{
									ClusterIP: corev1.ClusterIPNone,
								},
							},
						},
						StorageConfig: slinkyv1alpha1.StorageConfig{
							PasswordKeyRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb",
								},
								Key: "password",
							},
						},
					},
				},
			},
			wantClusterIP:                corev1.ClusterIPNone,
			wantPublishNotReadyAddresses: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildAccountingService(tt.args.accounting)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildAccountingService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got2, err := b.BuildAccounting(tt.args.accounting)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildAccounting() error = %v, wantErr %v", err, tt.wantErr)
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

			// Validate ClusterIP
			if got.Spec.ClusterIP != tt.wantClusterIP {
				t.Errorf("ClusterIP = %v, want %v", got.Spec.ClusterIP, tt.wantClusterIP)
			}

			// Validate PublishNotReadyAddresses
			if got.Spec.PublishNotReadyAddresses != tt.wantPublishNotReadyAddresses {
				t.Errorf("PublishNotReadyAddresses = %v, want %v",
					got.Spec.PublishNotReadyAddresses, tt.wantPublishNotReadyAddresses)
			}
		})
	}
}
