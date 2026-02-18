// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"path"
	"testing"
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
			if tt.wantFound && got == "" || !tt.wantFound && got != "" {
				t.Errorf("GetEnvTestBinary() = %v, wantFound %v", got, tt.wantFound)
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
			if len(got) != tt.args.length {
				t.Errorf("got wrong length: got = %v, want = %v", len(got), tt.args.length)
			}
		})
	}
}
