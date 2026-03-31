// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"crypto/rand"
	"fmt"
)

const (
	DefaultSigningKeyLength = 1024
)

func NewSigningKey() ([]byte, error) {
	return NewSigningKeyWithLength(DefaultSigningKeyLength)
}

func NewSigningKeyWithLength(length int) ([]byte, error) {
	key := make([]byte, length)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate signing key: %w", err)
	}
	return key, nil
}
