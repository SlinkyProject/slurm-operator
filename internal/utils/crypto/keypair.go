// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"fmt"

	"golang.org/x/crypto/ssh"
)

type KeyPairType string

const (
	KeyPairRsa     KeyPairType = "rsa"
	KeyPairEcdsa   KeyPairType = "ecdsa"
	KeyPairEd25519 KeyPairType = "ed25519"
)

type Option func(*KeyPair)

func WithType(keyPairType KeyPairType) Option {
	return func(o *KeyPair) {
		o.keyType = keyPairType
	}
}

func WithPassphrase(passphrase string) Option {
	return func(o *KeyPair) {
		o.passphrase = []byte(passphrase)
	}
}

func WithComment(comment string) Option {
	return func(o *KeyPair) {
		o.comment = comment
	}
}

func WithRsaLength(length int) Option {
	return func(o *KeyPair) {
		o.bitLength = length
	}
}

func WithEcdsaCurve(curve elliptic.Curve) Option {
	return func(o *KeyPair) {
		o.ellipticCurve = curve
	}
}

type KeyPair struct {
	keyType       KeyPairType
	privateKey    crypto.PrivateKey
	publicKey     crypto.PublicKey
	passphrase    []byte
	comment       string
	bitLength     int
	ellipticCurve elliptic.Curve
}

func NewKeyPair(opts ...Option) (*KeyPair, error) {
	o := &KeyPair{
		bitLength:     1024,
		ellipticCurve: elliptic.P256(),
		keyType:       KeyPairEd25519,
	}

	for _, opt := range opts {
		opt(o)
	}

	var err error
	switch o.keyType {
	case KeyPairRsa:
		err = o.generateRsa()
	case KeyPairEcdsa:
		err = o.generateEcdsa()
	case KeyPairEd25519:
		err = o.generateEd25519()
	default:
		err = fmt.Errorf("unsupported key pair type: %v", o.keyType)
	}
	if err != nil {
		return nil, err
	}

	return o, nil
}

// generateRsa generates an RSA keypair.
func (o *KeyPair) generateRsa() error {
	privateKey, err := rsa.GenerateKey(rand.Reader, o.bitLength)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key pair: %w", err)
	}
	err = privateKey.Validate()
	if err != nil {
		return err
	}
	o.privateKey = privateKey
	o.publicKey = privateKey.Public()
	return nil
}

// generateRsa generates an ECDSA keypair.
func (o *KeyPair) generateEcdsa() error {
	if o.ellipticCurve == nil {
		return errors.New("ECDSA curve is nil")
	}
	privateKey, err := ecdsa.GenerateKey(o.ellipticCurve, rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate ECDSA key pair: %w", err)
	}
	o.privateKey = privateKey
	o.publicKey = privateKey.Public()
	return nil
}

// generateRsa generates an ED25519 keypair.
func (o *KeyPair) generateEd25519() error {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate ED25519 key pair: %w", err)
	}
	o.privateKey = &privateKey
	o.publicKey = privateKey.Public()
	return nil
}

// PrivateKey returns the private SSH key in PEM format.
func (o *KeyPair) PrivateKey() ([]byte, error) {
	block, err := o.pemBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to create PEM block: %w", err)
	}
	return pem.EncodeToMemory(block), nil
}

// pemBlock returns the PEM key metadata block.
func (o *KeyPair) pemBlock() (*pem.Block, error) {
	if len(o.passphrase) > 0 {
		return ssh.MarshalPrivateKeyWithPassphrase(o.privateKey, o.comment, o.passphrase)
	}
	return ssh.MarshalPrivateKey(o.privateKey, o.comment)
}

// PublicKey returns the public SSH key in `authorized_keys` format.
func (o *KeyPair) PublicKey() ([]byte, error) {
	key, err := ssh.NewPublicKey(o.publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create public key: %w", err)
	}
	return ssh.MarshalAuthorizedKey(key), nil
}
