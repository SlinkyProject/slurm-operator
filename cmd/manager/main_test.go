// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"net"
	"net/http"
	"net/http/httptest"
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

func Test_parseFlags_profile(t *testing.T) {
	flags := parseFlagsForTest(t, []string{
		"test",
		"--profile",
		"--profile-addr",
		"localhost:6061",
		"--leader-elect",
	})

	require.True(t, flags.profile)
	require.Equal(t, "localhost:6061", flags.profileAddr)
	require.True(t, flags.enableLeaderElection)
}

func Test_profileAddr(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{
			name: "default",
			addr: "",
			want: defaultProfileAddr,
		},
		{
			name: "explicit",
			addr: "127.0.0.1:6061",
			want: "127.0.0.1:6061",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, profileAddr(tt.addr))
		})
	}
}

func Test_newProfileMux(t *testing.T) {
	server := httptest.NewServer(newProfileMux())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/debug/pprof/")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, resp.Body.Close())
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func Test_startProfileServer_bindError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, listener.Close())
	})

	_, err = startProfileServer(listener.Addr().String())
	require.Error(t, err)
}
