// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package reflectutils

import (
	"testing"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
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
			got := UseNonZeroOrDefault(tt.in, tt.def)
			if got != tt.want {
				t.Errorf("UseNonZeroOrDefault() = %v, want %v", got, tt.want)
			}
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
			got := UseNonZeroOrDefault(tt.in, tt.def)
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("UseNonZeroOrDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
