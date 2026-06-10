// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"crypto/elliptic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewKeyPair(t *testing.T) {
	type args struct {
		opts []Option
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "RSA",
			args: args{
				opts: []Option{
					WithType(KeyPairRsa),
				},
			},
		},
		{
			name: "RSA, with options",
			args: args{
				opts: []Option{
					WithType(KeyPairRsa),
					WithRsaLength(4096),
					WithPassphrase("foo"),
					WithComment("user@example.com"),
				},
			},
		},
		{
			name: "RSA, insecure length",
			args: args{
				opts: []Option{
					WithType(KeyPairRsa),
					WithRsaLength(256),
				},
			},
			wantErr: true,
		},
		{
			name: "Ecdsa",
			args: args{
				opts: []Option{
					WithType(KeyPairEcdsa),
				},
			},
		},
		{
			name: "Ecdsa, with options",
			args: args{
				opts: []Option{
					WithType(KeyPairEcdsa),
					WithEcdsaCurve(elliptic.P521()),
					WithPassphrase("foo"),
					WithComment("user@example.com"),
				},
			},
		},
		{
			name: "Ecdsa, invalid curve",
			args: args{
				opts: []Option{
					WithType(KeyPairEcdsa),
					WithEcdsaCurve(nil),
				},
			},
			wantErr: true,
		},
		{
			name: "Ed25519",
			args: args{
				opts: []Option{
					WithType(KeyPairEd25519),
				},
			},
		},
		{
			name: "Ed25519, with options",
			args: args{
				opts: []Option{
					WithType(KeyPairEd25519),
					WithPassphrase("foo"),
					WithComment("user@example.com"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewKeyPair(tt.args.opts...)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			privKey, err := got.PrivateKey()
			require.NoError(t, err)
			require.NotEmpty(t, privKey)

			pubKey, err := got.PublicKey()
			require.NoError(t, err)
			require.NotEmpty(t, pubKey)
		})
	}
}
