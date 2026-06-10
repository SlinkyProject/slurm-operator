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
	if flags.probeAddr != "8080" {
		t.Errorf("Test_parseFlags() probeAddr = %v, want %v", flags.probeAddr, "8080")
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
			flags := parseFlagsForTest(t, tt.args)
			if flags.namespaces != tt.want {
				t.Errorf("parseFlags() namespaces = %v, want %v", flags.namespaces, tt.want)
			}
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

	if !flags.profile {
		t.Errorf("parseFlags() profile = %v, want %v", flags.profile, true)
	}
	if flags.profileAddr != "localhost:6061" {
		t.Errorf("parseFlags() profileAddr = %v, want %v", flags.profileAddr, "localhost:6061")
	}
	if !flags.enableLeaderElection {
		t.Errorf("parseFlags() enableLeaderElection = %v, want %v", flags.enableLeaderElection, true)
	}
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
			if got := profileAddr(tt.addr); got != tt.want {
				t.Errorf("profileAddr(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func Test_newProfileMux(t *testing.T) {
	server := httptest.NewServer(newProfileMux())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("http.Get(%q) error = %v", server.URL+"/debug/pprof/", err)
	}
	t.Cleanup(func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Body.Close() error = %v", err)
		}
	})

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /debug/pprof/ status = %v, want %v", resp.StatusCode, http.StatusOK)
	}
}

func Test_startProfileServer_bindError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("listener.Close() error = %v", err)
		}
	})

	if _, err := startProfileServer(listener.Addr().String()); err == nil {
		t.Fatalf("startProfileServer(%q) error = nil, want non-nil", listener.Addr().String())
	}
}
