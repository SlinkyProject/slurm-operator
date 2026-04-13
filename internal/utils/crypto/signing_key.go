// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"crypto/rand"
)

const (
	DefaultSigningKeyLength = 1024
)

func NewSigningKey() []byte {
	return NewSigningKeyWithLength(DefaultSigningKeyLength)
}

func NewSigningKeyWithLength(length int) []byte {
	key := make([]byte, length)
	if _, err := rand.Read(key); err != nil {
		// rand.Read() signature returns an error, but in practice it never will
		// because it internally panics on error.
		//
		// Should a non-nil error ever be returned we panic here to ensure we
		// never return a dubious value on accident.
		panic(err)
	}
	return key
}
