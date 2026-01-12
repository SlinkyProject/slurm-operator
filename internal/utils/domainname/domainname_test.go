// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package domainname

import (
	"path"
	"testing"
)

func Test_clusterDomain(t *testing.T) {
	tests := []struct {
		name        string
		resolveConf string
		want        string
	}{
		{
			name:        "empty",
			resolveConf: "empty.conf",
			want:        "cluster.local",
		},
		{
			name:        "kubernetes",
			resolveConf: "kubernetes.conf",
			want:        "cluster.local",
		},
		{
			name:        "custom",
			resolveConf: "custom.conf",
			want:        "foo.local",
		},
		{
			name:        "malformed",
			resolveConf: "malformed.conf",
			want:        "cluster.local",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvConfPath = path.Join(".testdata", tt.resolveConf)
			got := clusterDomain()
			if got != tt.want {
				t.Errorf("clusterDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Benchmark_clusterDomain(b *testing.B) {
	benchmarks := []struct {
		name        string
		resolveConf string
	}{
		{
			name:        "empty",
			resolveConf: "empty.conf",
		},
		{
			name:        "kubernetes",
			resolveConf: "kubernetes.conf",
		},
		{
			name:        "custom",
			resolveConf: "custom.conf",
		},
		{
			name:        "malformed",
			resolveConf: "malformed.conf",
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			resolvConfPath = path.Join(".testdata", bb.resolveConf)
			for b.Loop() {
				clusterDomain()
			}
		})
	}
}
