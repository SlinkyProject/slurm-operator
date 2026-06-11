// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accountingbuilder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildAccountingService(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		accounting *slinkyv1beta1.Accounting
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
				accounting: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.AccountingSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
						StorageConfig: slinkyv1beta1.StorageConfig{
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
		},
		{
			name: "with nodeport",
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
				accounting: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.AccountingSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
						StorageConfig: slinkyv1beta1.StorageConfig{
							PasswordKeyRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb",
								},
								Key: "password",
							},
						},
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
							Name:       "slurmdbd",
							Protocol:   "TCP",
							Port:       6819,
							TargetPort: intstr.FromString("slurmdbd"),
							NodePort:   32500,
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "slurm",
						"app.kubernetes.io/name":     "slurmdbd",
					},
				},
			},
		},
		{
			name: "with external IPs",
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
				accounting: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.AccountingSpec{
						JwtKeyRef: &corev1.SecretKeySelector{},
						StorageConfig: slinkyv1beta1.StorageConfig{
							PasswordKeyRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mariadb",
								},
								Key: "password",
							},
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
							Name:       "slurmdbd",
							Protocol:   "TCP",
							Port:       6819,
							TargetPort: intstr.FromString("slurmdbd"),
						},
					},
					Selector: map[string]string{
						"app.kubernetes.io/instance": "slurm",
						"app.kubernetes.io/name":     "slurmdbd",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildAccountingService(tt.args.accounting)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			got2, err := b.BuildAccounting(tt.args.accounting)
			require.NoError(t, err)
			require.True(t, set.KeySet(got2.Labels).HasAll(set.KeySet(got.Spec.Selector).UnsortedList()...))
			require.True(t,
				got.Spec.Ports[0].TargetPort.String() == got2.Spec.Template.Spec.Containers[0].Ports[0].Name ||
					got.Spec.Ports[0].TargetPort.IntValue() == int(got2.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort))

			if tt.want != nil {
				require.Equal(t, tt.want.Spec, got.Spec)
			}
		})
	}
}
