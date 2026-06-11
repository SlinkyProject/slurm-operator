// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package reflectutils

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestUseNonZeroOrDefault_String(t *testing.T) {
	tests := []struct {
		name string
		in   string
		def  string
		want string
	}{
		{
			name: "zeroes",
		},
		{
			name: "non-zero",
			in:   "foo",
			def:  "bar",
			want: "foo",
		},
		{
			name: "default",
			def:  "foo",
			want: "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, UseNonZeroOrDefault(tt.in, tt.def))
		})
	}
}

func TestUseNonZeroOrDefault_Pointer(t *testing.T) {
	tests := []struct {
		name string
		in   *string
		def  *string
		want *string
	}{
		{
			name: "zeroes",
		},
		{
			name: "non-zero",
			in:   ptr.To("foo"),
			def:  ptr.To("bar"),
			want: ptr.To("foo"),
		},
		{
			name: "default",
			def:  ptr.To("foo"),
			want: ptr.To("foo"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, UseNonZeroOrDefault(tt.in, tt.def))
		})
	}
}
