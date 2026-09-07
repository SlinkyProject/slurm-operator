// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

// sshHostKey describes one SSH host keypair held in the host keys Secret.
type sshHostKey struct {
	keyType     crypto.KeyPairType
	privateFile string
	publicFile  string
	opts        []crypto.Option
}

var sshHostKeys = []sshHostKey{
	{
		keyType:     crypto.KeyPairRsa,
		privateFile: SshHostRsaKeyFile,
		publicFile:  SshHostRsaPubKeyFile,
		opts:        []crypto.Option{crypto.WithRsaLength(crypto.DefaultRsaBitLength)},
	},
	{
		keyType:     crypto.KeyPairEd25519,
		privateFile: SshHostEd25519KeyFile,
		publicFile:  SshHostEd25519PubKeyFile,
	},
	{
		keyType:     crypto.KeyPairEcdsa,
		privateFile: SshHostEcdsaKeyFile,
		publicFile:  SshHostEcdsaPubKeyFile,
	},
}

func (b *LoginBuilder) BuildLoginSshHostKeys(ctx context.Context, loginset *slinkyv1beta1.LoginSet) (*corev1.Secret, error) {
	// Reuse the keys already in the cluster. Generating new ones on every
	// reconcile is wasted work, and replacing a host key would invalidate the
	// one clients have already accepted.
	old := &corev1.Secret{}
	if err := b.client.Get(ctx, loginset.SshHostKeys(), old); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get object (%s): %w", klog.KObj(old), err)
		}
	}

	data := make(map[string][]byte, 2*len(sshHostKeys))
	for _, hostKey := range sshHostKeys {
		privateKey, publicKey := old.Data[hostKey.privateFile], old.Data[hostKey.publicFile]
		if len(privateKey) == 0 || len(publicKey) == 0 {
			var err error
			privateKey, publicKey, err = generateSshHostKey(hostKey)
			if err != nil {
				return nil, err
			}
		}
		data[hostKey.privateFile] = privateKey
		data[hostKey.publicFile] = publicKey
	}

	opts := common.SecretOpts{
		Key: loginset.SshHostKeys(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: loginset.Annotations,
			Labels:      structutils.MergeMaps(loginset.Labels, labels.NewBuilder().WithLoginLabels(loginset).Build()),
		},
		Data:      data,
		Immutable: true,
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithLoginLabels(loginset).Build())

	secret, err := b.CommonBuilder.BuildSecret(opts, loginset)
	if err != nil {
		return secret, fmt.Errorf("failed to build secret: %w", err)
	}

	return secret, nil
}

// generateSshHostKey returns a new private and public key for the given host key.
func generateSshHostKey(hostKey sshHostKey) ([]byte, []byte, error) {
	opts := append([]crypto.Option{crypto.WithType(hostKey.keyType)}, hostKey.opts...)
	keyPair, err := crypto.NewKeyPair(opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create %s key pair: %w", hostKey.keyType, err)
	}
	privateKey, err := keyPair.PrivateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode %s private key: %w", hostKey.keyType, err)
	}
	publicKey, err := keyPair.PublicKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode %s public key: %w", hostKey.keyType, err)
	}
	return privateKey, publicKey, nil
}
