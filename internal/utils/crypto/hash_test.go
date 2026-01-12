// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"testing"
)

func TestCheckSum(t *testing.T) {
	type args struct {
		b []byte
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{
				b: []byte{},
			},
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "non-empty",
			args: args{
				b: []byte("foo"),
			},
			want: "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckSum(tt.args.b); got != tt.want {
				t.Errorf("CheckSum() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkCheckSum(b *testing.B) {
	type args struct {
		b []byte
	}
	benchmarks := []struct {
		name string
		args args
	}{
		{
			name: "empty",
			args: args{
				b: []byte{},
			},
		},
		{
			name: "non-empty",
			args: args{
				b: []byte("foo"),
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				CheckSum(bb.args.b)
			}
		})
	}
}

func TestCheckSumFromMap(t *testing.T) {
	type args struct {
		items map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{
				items: map[string]string{},
			},
			want: "",
		},
		{
			name: "non-empty",
			args: args{
				items: map[string]string{
					"foo":  "bar",
					"fizz": "buzz",
				},
			},
			want: "093635f9ad1c31773993253f0daf910da63189bb2cc120e0c89fbf2b57bb05fe",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckSumFromMap(tt.args.items); got != tt.want {
				t.Errorf("CheckSumFromMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkCheckSumFromMap(b *testing.B) {
	type args struct {
		items map[string]string
	}
	benchmarks := []struct {
		name string
		args args
	}{
		{
			name: "empty",
			args: args{
				items: map[string]string{},
			},
		},
		{
			name: "non-empty",
			args: args{
				items: map[string]string{
					"foo":  "bar",
					"fizz": "buzz",
				},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				CheckSumFromMap(bb.args.items)
			}
		})
	}
}
