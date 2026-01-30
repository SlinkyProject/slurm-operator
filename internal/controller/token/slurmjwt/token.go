// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmjwt

import (
	"fmt"
	"math"
	"time"

	"github.com/togethercomputer/slurm-operator/internal/utils/mathutils"
	jwt "github.com/golang-jwt/jwt/v5"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	// Slurm defines `lifetime=infinite` as this.
	// Ref: https://github.com/SchedMD/slurm/blob/master/src/scontrol/scontrol.c#L1003
	infinite = math.MaxInt32 - 1
)

type Token struct {
	signingKey []byte
	method     jwt.SigningMethod
	username   string
	lifetime   time.Duration
}

func NewToken(signingKey []byte) *Token {
	return &Token{
		signingKey: signingKey,
		method:     jwt.SigningMethodHS256,
		username:   "slurm",
		lifetime:   infinite * time.Second,
	}
}

func (t *Token) WithLifetime(lifetime time.Duration) *Token {
	t.lifetime = mathutils.Clamp(lifetime, 0, infinite*time.Second)
	return t
}

func (t *Token) WithUsername(username string) *Token {
	t.username = username
	return t
}

// Ref: https://slurm.schedmd.com/jwt.html#compatibility
type TokenClaims struct {
	jwt.RegisteredClaims `json:",inline"`

	SlurmUsername string `json:"sun"`
}

func (t *Token) NewSignedToken() (string, error) {
	now := time.Now()
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        string(uuid.NewUUID()),
			Issuer:    "slurm-operator",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.lifetime)),
			NotBefore: jwt.NewNumericDate(now),
		},
		SlurmUsername: t.username,
	}

	token := jwt.NewWithClaims(t.method, claims)

	tokenString, err := token.SignedString(t.signingKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func ParseTokenClaims(tokenString string, signingKey []byte) (jwt.MapClaims, error) {
	signingKeyFunc := func(token *jwt.Token) (any, error) {
		return signingKey, nil
	}

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, signingKeyFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return claims, nil
}

func VerifyToken(tokenString string, signingKey []byte) (bool, error) {
	signingKeyFunc := func(token *jwt.Token) (any, error) {
		return signingKey, nil
	}

	token, err := jwt.Parse(tokenString, signingKeyFunc)
	if err != nil {
		return false, fmt.Errorf("failed to parse JWT: %w", err)
	}

	return token.Valid, nil
}
