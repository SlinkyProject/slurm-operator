// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmjwt

import (
	"testing"
	"time"

	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
)

func mustNewSigningKey(t *testing.T) []byte {
	t.Helper()
	key, err := crypto.NewSigningKey()
	if err != nil {
		t.Fatalf("NewSigningKey() error = %v", err)
	}
	return key
}

func newSignedToken(signingKey []byte) string {
	tokenString, err := NewToken(signingKey).NewSignedToken()
	if err != nil {
		panic(err)
	}
	return tokenString
}

func TestToken_NewSignedToken(t *testing.T) {
	signingKey1 := mustNewSigningKey(t)
	signingKey2 := mustNewSigningKey(t)
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
				token: NewToken(signingKey1),
			},
			wantOk: true,
		},
		{
			name: "With Options",
			fields: fields{
				token: NewToken(signingKey2).
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Token.NewSignedToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			ok, err := VerifyToken(got, tr.signingKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyToken() = %v", err)
				return
			}
			if ok != tt.wantOk {
				t.Errorf("VerifyToken() ok = %v, wantOk %v", ok, tt.wantOk)
			}
		})
	}
}

func TestParseTokenClaims(t *testing.T) {
	signingKey := mustNewSigningKey(t)
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
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTokenClaims() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestVerifyToken(t *testing.T) {
	signingKey := mustNewSigningKey(t)
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
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if ok != tt.wantOk {
				t.Errorf("VerifyToken() ok = %v, wantOk %v", ok, tt.wantOk)
			}
		})
	}
}
