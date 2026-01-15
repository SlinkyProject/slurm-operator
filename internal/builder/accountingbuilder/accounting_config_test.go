// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accountingbuilder

import (
	"strings"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	common "github.com/SlinkyProject/slurm-operator/internal/builder/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildAccountingConfig(t *testing.T) {
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
						ExtraConf: strings.Join([]string{
							"CommitDelay=1",
						}, "\n"),
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildAccountingConfig(tt.args.accounting)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildAccountingConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case got.Data[common.SlurmdbdConfFile] == nil && got.StringData[common.SlurmdbdConfFile] == "":
				t.Errorf("got.Data[%s] = %v", common.SlurmdbdConfFile, got.Data[common.SlurmdbdConfFile])
			}
		})
	}
}

func BenchmarkBuilder_BuildAccountingConfig(b *testing.B) {
	type fields struct {
		client client.Client
	}
	type args struct {
		accounting *slinkyv1beta1.Accounting
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
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
						ExtraConf: strings.Join([]string{
							"CommitDelay=1",
						}, "\n"),
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
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			build := New(bb.fields.client)

			for b.Loop() {
				build.BuildAccountingConfig(bb.args.accounting) //nolint:errcheck
			}
		})
	}
}
