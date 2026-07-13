// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmjwt

import (
	"testing"
	"time"

	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
	"github.com/stretchr/testify/require"
)

func newSignedToken(signingKey []byte) string {
	tokenString, err := NewToken(signingKey).NewSignedToken()
	if err != nil {
		panic(err)
	}
	return tokenString
}

func TestToken_NewSignedToken(t *testing.T) {
	type fields struct {
		token *Token
	}
	tests := []struct {
		name    string
		fields  fields
		wantOk  bool
		wantErr bool
	}{
		{
			name: "Generate",
			fields: fields{
				token: NewToken(crypto.NewSigningKey()),
			},
			wantOk: true,
		},
		{
			name: "With Options",
			fields: fields{
				token: NewToken(crypto.NewSigningKey()).
					WithUsername("foo").
					WithLifetime(30 * time.Second),
			},
			wantOk: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.fields.token
			got, err := tr.NewSignedToken()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			ok, err := VerifyToken(got, tr.signingKey)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantOk, ok)
		})
	}
}

func TestParseTokenClaims(t *testing.T) {
	signingKey := crypto.NewSigningKey()
	type args struct {
		tokenString string
		signingKey  []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "empty",
			args: args{
				tokenString: newSignedToken(signingKey),
				signingKey:  signingKey,
			},
		},
		{
			name: "different signingKey",
			args: args{
				tokenString: newSignedToken(signingKey),
				signingKey:  nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTokenClaims(tt.args.tokenString, tt.args.signingKey)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVerifyToken(t *testing.T) {
	signingKey := crypto.NewSigningKey()
	type args struct {
		tokenString string
		signingKey  []byte
	}
	tests := []struct {
		name    string
		args    args
		wantOk  bool
		wantErr bool
	}{
		{
			name: "Empty Token",
			args: args{
				tokenString: "",
				signingKey:  signingKey,
			},
			wantErr: true,
		},
		{
			name: "Valid",
			args: args{
				tokenString: newSignedToken(signingKey),
				signingKey:  signingKey,
			},
			wantOk: true,
		},
		{
			name: "Different SigningKey",
			args: args{
				tokenString: newSignedToken(signingKey),
				signingKey:  nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := VerifyToken(tt.args.tokenString, tt.args.signingKey)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantOk, ok)
		})
	}
}
