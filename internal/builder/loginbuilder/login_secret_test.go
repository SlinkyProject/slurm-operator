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

// The host keys Secret is the source of truth once it exists: regenerating the
// keys would invalidate the host keys clients have already accepted.
func TestBuilder_BuildLoginSshHostKeys_reusesExistingKeys(t *testing.T) {
	loginset := &slinkyv1beta1.LoginSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	key := loginset.SshHostKeys()

	existing := map[string][]byte{}
	for _, file := range []string{
		SshHostRsaKeyFile, SshHostRsaPubKeyFile,
		SshHostEd25519KeyFile, SshHostEd25519PubKeyFile,
		SshHostEcdsaKeyFile, SshHostEcdsaPubKeyFile,
	} {
		existing[file] = []byte("existing-" + file)
	}

	c := fake.NewFakeClient(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
		Data: existing,
	})

	got, err := New(c).BuildLoginSshHostKeys(context.TODO(), loginset)
	require.NoError(t, err)
	require.Equal(t, existing, got.Data)
}
