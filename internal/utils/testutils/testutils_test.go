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

func BenchmarkGetEnvTestBinary(b *testing.B) {
	type args struct {
		rootPath string
	}
	benchmarks := []struct {
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
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				GetEnvTestBinary(bb.args.rootPath)
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

func BenchmarkGenerateResourceName(b *testing.B) {
	type args struct {
		length int
	}
	benchmarks := []struct {
		name string
		args args
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
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				GenerateResourceName(bb.args.length)
			}
		})
	}
}
