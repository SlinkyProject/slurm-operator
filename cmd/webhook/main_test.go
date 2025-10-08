// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package main

import (
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
