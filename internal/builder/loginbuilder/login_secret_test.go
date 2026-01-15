// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildLoginSshHostKeys(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		loginset *slinkyv1beta1.LoginSet
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
				loginset: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildLoginSshHostKeys(tt.args.loginset)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildLoginSshHostKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case got.Data[SshHostEcdsaKeyFile] == nil && got.StringData[SshHostEcdsaKeyFile] == "":
				t.Errorf("got.Data[%s] = %v", SshHostEcdsaKeyFile, got.Data[SshHostEcdsaKeyFile])
			case got.Data[SshHostEcdsaPubKeyFile] == nil && got.StringData[SshHostEcdsaPubKeyFile] == "":
				t.Errorf("got.Data[%s] = %v", SshHostEcdsaPubKeyFile, got.Data[SshHostEcdsaPubKeyFile])

			case got.Data[SshHostEd25519KeyFile] == nil && got.StringData[SshHostEd25519KeyFile] == "":
				t.Errorf("got.Data[%s] = %v", SshHostEd25519KeyFile, got.Data[SshHostEd25519KeyFile])
			case got.Data[SshHostEd25519PubKeyFile] == nil && got.StringData[SshHostEd25519PubKeyFile] == "":
				t.Errorf("got.Data[%s] = %v", SshHostEd25519PubKeyFile, got.Data[SshHostEd25519PubKeyFile])

			case got.Data[SshHostRsaKeyFile] == nil && got.StringData[SshHostRsaKeyFile] == "":
				t.Errorf("got.Data[%s] = %v", SshHostRsaKeyFile, got.Data[SshHostRsaKeyFile])
			case got.Data[SshHostRsaPubKeyFile] == nil && got.StringData[SshHostRsaPubKeyFile] == "":
				t.Errorf("got.Data[%s] = %v", SshHostRsaPubKeyFile, got.Data[SshHostRsaPubKeyFile])
			}
		})
	}
}

func BenchmarkBuilder_BuildLoginSshHostKeys(b *testing.B) {
	type fields struct {
		client client.Client
	}
	type args struct {
		loginset *slinkyv1beta1.LoginSet
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				loginset: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			build := New(bb.fields.client)

			for b.Loop() {
				build.BuildLoginSshHostKeys(bb.args.loginset) //nolint:errcheck
			}
		})
	}
}
