// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	_ "embed"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildTokenSecret(t *testing.T) {
	jwtHs256Secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm-jwths256key",
		},
		Data: map[string][]byte{
			"jwt_hs256.key": []byte("foo"),
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
					WithObjects(jwtHs256Secret).
					Build(),
			},
			args: args{
				token: &slinkyv1beta1.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.TokenSpec{
						Username: "foo",
						JwtHs256KeyRef: slinkyv1beta1.JwtSecretKeySelector{
							SecretKeySelector: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "slurm-jwths256key",
								},
								Key: "jwt_hs256.key",
							},
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
						JwtHs256KeyRef: slinkyv1beta1.JwtSecretKeySelector{
							SecretKeySelector: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "slurm-jwths256key",
								},
								Key: "jwt_hs256.key",
							},
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
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildTokenSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case ptr.Deref(got.Immutable, false) != !tt.args.token.Spec.Refresh:
				t.Errorf("Immutable = %v , want = %v",
					ptr.Deref(got.Immutable, false), !tt.args.token.Spec.Refresh)
			}
		})
	}
}

func BenchmarkBuilder_BuildTokenSecret(b *testing.B) {
	jwtHs256Secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm-jwths256key",
		},
		Data: map[string][]byte{
			"jwt_hs256.key": []byte("foo"),
		},
	}
	type fields struct {
		client client.Client
	}
	type args struct {
		token *slinkyv1beta1.Token
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(jwtHs256Secret).
					Build(),
			},
			args: args{
				token: &slinkyv1beta1.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.TokenSpec{
						Username: "foo",
						JwtHs256KeyRef: slinkyv1beta1.JwtSecretKeySelector{
							SecretKeySelector: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "slurm-jwths256key",
								},
								Key: "jwt_hs256.key",
							},
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
						JwtHs256KeyRef: slinkyv1beta1.JwtSecretKeySelector{
							SecretKeySelector: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "slurm-jwths256key",
								},
								Key: "jwt_hs256.key",
							},
						},
					},
				},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			build := New(bb.fields.client)

			for b.Loop() {
				build.BuildTokenSecret(bb.args.token) //nolint:errcheck
			}
		})
	}
}
