// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/stretchr/testify/require"
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

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.True(t, got.Data[SshHostEcdsaKeyFile] != nil || got.StringData[SshHostEcdsaKeyFile] != "")
			require.True(t, got.Data[SshHostEcdsaPubKeyFile] != nil || got.StringData[SshHostEcdsaPubKeyFile] != "")
			require.True(t, got.Data[SshHostEd25519KeyFile] != nil || got.StringData[SshHostEd25519KeyFile] != "")
			require.True(t, got.Data[SshHostEd25519PubKeyFile] != nil || got.StringData[SshHostEd25519PubKeyFile] != "")
			require.True(t, got.Data[SshHostRsaKeyFile] != nil || got.StringData[SshHostRsaKeyFile] != "")
			require.True(t, got.Data[SshHostRsaPubKeyFile] != nil || got.StringData[SshHostRsaPubKeyFile] != "")
		})
	}
}
