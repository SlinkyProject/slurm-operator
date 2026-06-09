// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func parseFlagsForTest(t *testing.T, args []string) Flags {
	t.Helper()

	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	t.Cleanup(func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	})

	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)
	os.Args = args

	var flags Flags
	parseFlags(&flags)
	return flags
}

func Test_parseFlags(t *testing.T) {
	flags := parseFlagsForTest(t, []string{"test", "--health-addr", "8080", "--leader-elect", "true"})

	require.Equal(t, "8080", flags.probeAddr)
	require.True(t, flags.enableLeaderElection)
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
			flags := parseFlagsForTest(t, tt.args)
			require.Equal(t, tt.want, flags.namespaces)
		})
	}
}
