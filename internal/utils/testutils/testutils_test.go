// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetEnvTestBinary(t *testing.T) {
	type args struct {
		rootPath string
	}
	tests := []struct {
		name      string
		args      args
		wantFound bool
	}{
		{
			name: "Wrong",
			args: args{
				rootPath: "",
			},
			wantFound: false,
		},
		{
			name: "Found",
			args: args{
				rootPath: path.Join("..", "..", ".."),
			},
			wantFound: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEnvTestBinary(tt.args.rootPath)

			if tt.wantFound {
				require.NotEmpty(t, got)
			} else {
				require.Empty(t, got)
			}
		})
	}
}

func TestGenerateResourceName(t *testing.T) {
	type args struct {
		length int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "min",
			args: args{
				length: 1,
			},
		},
		{
			name: "max",
			args: args{
				length: 63,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateResourceName(tt.args.length)

			require.Len(t, got, tt.args.length)
		})
	}
}
