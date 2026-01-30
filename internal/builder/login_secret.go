// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/builder/labels"
	"github.com/togethercomputer/slurm-operator/internal/utils/crypto"
	"github.com/togethercomputer/slurm-operator/internal/utils/structutils"
)

func (b *Builder) BuildLoginSshHostKeys(loginset *slinkyv1beta1.LoginSet) (*corev1.Secret, error) {
	keyPairRsa, err := crypto.NewKeyPair(crypto.WithType(crypto.KeyPairRsa))
	if err != nil {
		return nil, fmt.Errorf("failed to create RSA key pair: %w", err)
	}
	keyPairEd25519, err := crypto.NewKeyPair(crypto.WithType(crypto.KeyPairEd25519))
	if err != nil {
		return nil, fmt.Errorf("failed to create ED25519 key pair: %w", err)
	}
	keyPairEcdsa, err := crypto.NewKeyPair(crypto.WithType(crypto.KeyPairEcdsa))
	if err != nil {
		return nil, fmt.Errorf("failed to create ECDSA key pair: %w", err)
	}

	opts := SecretOpts{
		Key:      loginset.SshHostKeys(),
		Metadata: loginset.Spec.Template.PodMetadata,
		Data: map[string][]byte{
			sshHostEcdsaKeyFile:      keyPairRsa.PrivateKey(),
			sshHostEcdsaPubKeyFile:   keyPairRsa.PublicKey(),
			sshHostEd25519KeyFile:    keyPairEd25519.PrivateKey(),
			sshHostEd25519PubKeyFile: keyPairEd25519.PublicKey(),
			sshHostRsaKeyFile:        keyPairEcdsa.PrivateKey(),
			sshHostRsaPubKeyFile:     keyPairEcdsa.PublicKey(),
		},
		Immutable: true,
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithLoginLabels(loginset).Build())

	return b.BuildSecret(opts, loginset)
}
