// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"crypto/elliptic"
	"testing"
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
			if (err != nil) != tt.wantErr {
				t.Errorf("NewKeyPair() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if key := got.PrivateKey(); len(key) == 0 {
				t.Errorf("PrivateKey() len() = %v", len(key))
				return
			}
			if key := got.PublicKey(); len(key) == 0 {
				t.Errorf("PublicKey() len() = %v", len(key))
				return
			}
		})
	}
}
