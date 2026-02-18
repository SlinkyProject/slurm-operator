// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"testing"
)

func TestNewSigningKeyWithLength(t *testing.T) {
	type args struct {
		length int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Default",
			args: args{
				length: DefaultSigningKeyLength,
			},
		},
		{
			name: "Zero",
			args: args{
				length: 0,
			},
		},
		{
			name: "Small",
			args: args{
				length: 32,
			},
		},
		{
			name: "Large",
			args: args{
				length: 4096,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSigningKeyWithLength(tt.args.length)
			if len(got) != tt.args.length {
				t.Errorf("NewSigningKeyWithLength(): length = %v, got %v", tt.args.length, len(got))
			}
		})
	}
}
