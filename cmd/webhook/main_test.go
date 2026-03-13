// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"
	"testing"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func Test_parseFlags(t *testing.T) {
	flags := Flags{}
	os.Args = []string{"test", "--health-addr", "8080", "--leader-elect", "true"}
	parseFlags(&flags)
	if flags.probeAddr != "8080" {
		t.Errorf("Test_parseFlags() metricsAddr = %v, want %v", flags.probeAddr, "8080")
	}
	if !flags.enableLeaderElection {
		t.Errorf("Test_parseFlags() server = %v, want %v", flags.enableLeaderElection, true)
	}
}

func Test_parseFlags_namespaces(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "default is empty (all namespaces)",
			args: []string{"test"},
			want: "",
		},
		{
			name: "single namespace",
			args: []string{"test", "--namespaces", "slurm-system"},
			want: "slurm-system",
		},
		{
			name: "multiple namespaces",
			args: []string{"test", "--namespaces", "slurm-system,production,staging"},
			want: "slurm-system,production,staging",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(tt.args[0], flag.ContinueOnError)
			os.Args = tt.args
			flags := Flags{}
			parseFlags(&flags)
			if flags.namespaces != tt.want {
				t.Errorf("parseFlags() namespaces = %v, want %v", flags.namespaces, tt.want)
			}
		})
	}
}
