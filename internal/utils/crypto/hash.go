// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/togethercomputer/slurm-operator/internal/utils/structutils"
)

func CheckSum(b []byte) string {
	hash := sha256.Sum256(b)
	return fmt.Sprintf("%x", hash)
}

func CheckSumFromMap[T string | []byte](items map[string]T) string {
	keys := structutils.Keys(items)
	sort.Strings(keys)

	b := []byte{}
	for _, k := range keys {
		val := items[k]
		b = append(b, val...)
	}

	if len(b) == 0 {
		return ""
	}

	return CheckSum(b)
}
