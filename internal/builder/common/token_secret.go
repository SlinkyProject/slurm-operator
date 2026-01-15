// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/token/slurmjwt"
)

func (b *CommonBuilder) BuildTokenSecret(token *slinkyv1beta1.Token) (*corev1.Secret, error) {
	ctx := context.TODO()

	jwtHs256Ref := token.JwtHs256Ref()
	signingKey, err := b.refResolver.GetSecretKeyRef(ctx, &jwtHs256Ref.SecretKeySelector, jwtHs256Ref.Namespace)
	if err != nil {
		return nil, err
	}

	authToken, err := slurmjwt.NewToken(signingKey).
		WithUsername(token.Username()).
		WithLifetime(token.Lifetime()).
		NewSignedToken()
	if err != nil {
		return nil, fmt.Errorf("failed to create Slurm auth token: %w", err)
	}

	opts := SecretOpts{
		Key: token.SecretKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: token.Annotations,
			Labels:      token.Labels,
		},
		StringData: map[string]string{
			token.SecretRef().Key: authToken,
		},
		Immutable: !token.Spec.Refresh,
	}

	jwtHs256Secret := &corev1.Secret{}
	if err := b.client.Get(ctx, token.JwtHs256Key(), jwtHs256Secret); err != nil {
		return nil, err
	}

	o, err := b.BuildSecret(opts, jwtHs256Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to build token secret: %w", err)
	}

	return o, nil
}
