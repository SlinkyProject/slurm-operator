// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
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
			got, err := b.BuildLoginSshHostKeys(context.TODO(), tt.args.loginset)

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

// The host keys Secret is the source of truth once it exists: regenerating a
// keypair would invalidate the host key clients have already accepted.
func TestBuilder_BuildLoginSshHostKeys_reusesExistingKeys(t *testing.T) {
	loginset := &slinkyv1beta1.LoginSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}

	existingData := func(files ...string) map[string][]byte {
		data := map[string][]byte{}
		for _, file := range files {
			data[file] = []byte("existing-" + file)
		}
		return data
	}

	tests := []struct {
		name string
		// data of the live Secret; nil means the Secret does not exist
		existing map[string][]byte
		// files expected to keep their existing value
		wantReused []string
		// files expected to hold freshly generated material
		wantGenerated []string
	}{
		{
			name: "all key types present are reused",
			existing: existingData(
				SshHostRsaKeyFile, SshHostRsaPubKeyFile,
				SshHostEd25519KeyFile, SshHostEd25519PubKeyFile,
				SshHostEcdsaKeyFile, SshHostEcdsaPubKeyFile,
			),
			wantReused: []string{
				SshHostRsaKeyFile, SshHostRsaPubKeyFile,
				SshHostEd25519KeyFile, SshHostEd25519PubKeyFile,
				SshHostEcdsaKeyFile, SshHostEcdsaPubKeyFile,
			},
		},
		{
			name: "a missing key type is generated without disturbing the others",
			existing: existingData(
				SshHostRsaKeyFile, SshHostRsaPubKeyFile,
				SshHostEd25519KeyFile, SshHostEd25519PubKeyFile,
			),
			wantReused: []string{
				SshHostRsaKeyFile, SshHostRsaPubKeyFile,
				SshHostEd25519KeyFile, SshHostEd25519PubKeyFile,
			},
			wantGenerated: []string{SshHostEcdsaKeyFile, SshHostEcdsaPubKeyFile},
		},
		{
			name:          "a half-written key type is regenerated",
			existing:      existingData(SshHostRsaKeyFile),
			wantGenerated: []string{SshHostRsaKeyFile, SshHostRsaPubKeyFile},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: loginset.SshHostKeys().Namespace,
					Name:      loginset.SshHostKeys().Name,
				},
				Data: tt.existing,
			}
			c := fake.NewFakeClient(existing)

			got, err := New(c).BuildLoginSshHostKeys(context.TODO(), loginset)
			require.NoError(t, err)

			for _, file := range tt.wantReused {
				require.Equal(t, string(tt.existing[file]), string(got.Data[file]), file)
			}
			for _, file := range tt.wantGenerated {
				require.NotEmpty(t, got.Data[file], file)
				require.NotEqual(t, string(tt.existing[file]), string(got.Data[file]), file)
			}
		})
	}
}
