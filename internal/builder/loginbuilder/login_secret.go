// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func (b *LoginBuilder) BuildLoginSshHostKeys(loginset *slinkyv1beta1.LoginSet) (*corev1.Secret, error) {
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

	ecdsaPriv, err := keyPairEcdsa.PrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to encode ECDSA private key: %w", err)
	}
	ecdsaPub, err := keyPairEcdsa.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to encode ECDSA public key: %w", err)
	}
	ed25519Priv, err := keyPairEd25519.PrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to encode ED25519 private key: %w", err)
	}
	ed25519Pub, err := keyPairEd25519.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to encode ED25519 public key: %w", err)
	}
	rsaPriv, err := keyPairRsa.PrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to encode RSA private key: %w", err)
	}
	rsaPub, err := keyPairRsa.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to encode RSA public key: %w", err)
	}

	opts := common.SecretOpts{
		Key: loginset.SshHostKeys(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: loginset.Annotations,
			Labels:      structutils.MergeMaps(loginset.Labels, labels.NewBuilder().WithLoginLabels(loginset).Build()),
		},
		Data: map[string][]byte{
			SshHostEcdsaKeyFile:      ecdsaPriv,
			SshHostEcdsaPubKeyFile:   ecdsaPub,
			SshHostEd25519KeyFile:    ed25519Priv,
			SshHostEd25519PubKeyFile: ed25519Pub,
			SshHostRsaKeyFile:        rsaPriv,
			SshHostRsaPubKeyFile:     rsaPub,
		},
		Immutable: true,
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithLoginLabels(loginset).Build())

	return b.CommonBuilder.BuildSecret(opts, loginset)
}
