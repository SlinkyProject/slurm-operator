// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package reflectutils

import (
	"reflect"
)

// UseNonZeroOrDefault returns the input if not effectively zero,
// otherwise returns the default.
func UseNonZeroOrDefault[T any](in T, def T) T {
	if IsEmpty(in) {
		return def
	}
	return in
}

// IsEmpty returns true if the input value is effectively empty for its type.
func IsEmpty[T any](in T) bool {
	zero := reflect.Zero(reflect.TypeOf(in)).Interface()
	isZero := reflect.DeepEqual(in, zero)
	return isZero
}
