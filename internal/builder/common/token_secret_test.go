// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	_ "embed"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildTokenSecret(t *testing.T) {
	jwtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm-jwtkey",
		},
		Data: map[string][]byte{
			"jwt.key": []byte("foo"),
		},
	}
	type fields struct {
		client client.Client
	}
	type args struct {
		token *slinkyv1beta1.Token
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
					WithObjects(jwtSecret).
					Build(),
			},
			args: args{
				token: &slinkyv1beta1.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.TokenSpec{
						Username: "foo",
						JwtKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "slurm-jwtkey",
							},
							Key: "jwt.key",
						},
					},
				},
			},
		},
		{
			name: "not found",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				token: &slinkyv1beta1.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.TokenSpec{
						Username: "foo",
						JwtKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "slurm-jwtkey",
							},
							Key: "jwt.key",
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildTokenSecret(tt.args.token)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			refresh := ptr.Deref(tt.args.token.Spec.Refresh, defaults.DefaultTokenRefresh)
			require.Equal(t, !refresh, ptr.Deref(got.Immutable, false))
		})
	}
}
