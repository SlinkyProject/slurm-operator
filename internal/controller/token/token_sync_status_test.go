// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/token/slurmjwt"
	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
)

func TestTokenReconciler_syncStatus(t *testing.T) {
	signingKey := crypto.NewSigningKey()
	signedToken, err := slurmjwt.NewToken(signingKey).NewSignedToken()
	if err != nil {
		t.Fatalf("failed to create signed token: %v", err)
	}

	jwtKeySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-jwtkey",
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"jwt.key": signingKey,
		},
	}

	token := &slinkyv1beta1.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1beta1.TokenSpec{
			Username: "slurm",
			JwtKeyRef: &slinkyv1beta1.JwtSecretKeySelector{
				SecretKeySelector: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: jwtKeySecret.Name,
					},
					Key: "jwt.key",
				},
			},
		},
	}

	authSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      token.SecretKey().Name,
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			"SLURM_JWT": []byte(signedToken),
		},
	}

	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx   context.Context
		token *slinkyv1beta1.Token
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Succeeds with valid JWT and updates status",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithRuntimeObjects(token.DeepCopy(), jwtKeySecret.DeepCopy(), authSecret.DeepCopy()).
					WithStatusSubresource(&slinkyv1beta1.Token{}).
					Build(),
			},
			args: args{
				ctx:   context.TODO(),
				token: token.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "Fails when auth secret is missing",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithRuntimeObjects(token.DeepCopy(), jwtKeySecret.DeepCopy()).
					WithStatusSubresource(&slinkyv1beta1.Token{}).
					Build(),
			},
			args: args{
				ctx:   context.TODO(),
				token: token.DeepCopy(),
			},
			wantErr: true,
		},
		{
			name: "Fails when JWT signing key secret is missing",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithRuntimeObjects(token.DeepCopy(), authSecret.DeepCopy()).
					WithStatusSubresource(&slinkyv1beta1.Token{}).
					Build(),
			},
			args: args{
				ctx:   context.TODO(),
				token: token.DeepCopy(),
			},
			wantErr: true,
		},
		{
			name: "Fails when JWT is signed with wrong key",
			fields: fields{
				Client: func() client.Client {
					wrongKeySecret := jwtKeySecret.DeepCopy()
					wrongKeySecret.Data["jwt.key"] = crypto.NewSigningKey()
					return fake.NewClientBuilder().
						WithRuntimeObjects(token.DeepCopy(), wrongKeySecret, authSecret.DeepCopy()).
						WithStatusSubresource(&slinkyv1beta1.Token{}).
						Build()
				}(),
			},
			args: args{
				ctx:   context.TODO(),
				token: token.DeepCopy(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReconciler(tt.fields.Client)
			if err := r.syncStatus(tt.args.ctx, tt.args.token); (err != nil) != tt.wantErr {
				t.Errorf("TokenReconciler.syncStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTokenReconciler_updateStatus(t *testing.T) {
	token := &slinkyv1beta1.Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: corev1.NamespaceDefault,
		},
	}

	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx       context.Context
		token     *slinkyv1beta1.Token
		newStatus *slinkyv1beta1.TokenStatus
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Succeeds updating status",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithRuntimeObjects(token.DeepCopy()).
					WithStatusSubresource(&slinkyv1beta1.Token{}).
					Build(),
			},
			args: args{
				ctx:   context.TODO(),
				token: token.DeepCopy(),
				newStatus: &slinkyv1beta1.TokenStatus{
					IssuedAt: &metav1.Time{Time: metav1.Now().Time},
				},
			},
			wantErr: false,
		},
		{
			name: "No-op when token is not found",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				ctx:   context.TODO(),
				token: token.DeepCopy(),
				newStatus: &slinkyv1beta1.TokenStatus{
					IssuedAt: &metav1.Time{Time: metav1.Now().Time},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReconciler(tt.fields.Client)
			if err := r.updateStatus(tt.args.ctx, tt.args.token, tt.args.newStatus); (err != nil) != tt.wantErr {
				t.Errorf("TokenReconciler.updateStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
