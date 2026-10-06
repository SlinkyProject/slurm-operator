// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFailureDiagnosticsUseSelectedContext(t *testing.T) {
	dir := t.TempDir()
	trace := filepath.Join(dir, "kubectl-commands.txt")
	// Record every invocation, including pod discovery and commands inside pods.
	kubectl := `#!/bin/sh
printf '%s ' "$@" >> "$KUBECTL_TRACE"
printf '\n' >> "$KUBECTL_TRACE"
case " $* " in
  *" get pods "*" -o name "*) printf 'pod/diagnostic-probe\n' ;;
esac
`
	// The mock kubectl must be executable and is isolated in t.TempDir().
	if err := os.WriteFile(filepath.Join(dir, "kubectl"), []byte(kubectl), 0o700); err != nil { //nolint:gosec // the temporary mock must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("KUBECTL_TRACE", trace)
	t.Setenv(E2EKubeContextEnvironment, "e2e-selected")
	t.Setenv("E2E_ARTIFACTS_DIR", dir)

	CaptureFailureDiagnostics(t, "context regression", "slurm")

	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	commands := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, command := range commands {
		if !strings.HasPrefix(command, "--context e2e-selected ") {
			t.Errorf("diagnostic command can fall back to the current context: %s", command)
		}
	}
	for _, want := range []string{
		"get pods --namespace slurm -o name",
		"logs --namespace slurm pod/diagnostic-probe",
		"--previous",
		"sinfo --Node --long",
		"scontrol show nodes --details",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("diagnostics did not invoke %q", want)
		}
	}

	artifactDir := filepath.Join(dir, "failures", artifactName(t.Name()), "slurm")
	for _, filename := range []string{"slurm-nodes.txt", "diagnostic-probe.log"} {
		artifact, err := os.ReadFile(filepath.Join(artifactDir, filename))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(artifact), "$ kubectl --context e2e-selected ") {
			t.Errorf("%s does not record the selected context: %s", filename, artifact)
		}
	}
}

func TestRunKubectlRequiresContext(t *testing.T) {
	t.Setenv(E2EKubeContextEnvironment, "")
	t.Setenv("PATH", t.TempDir())

	if _, err := runKubectl("get", "nodes"); err == nil || !strings.Contains(err.Error(), E2EKubeContextEnvironment) {
		t.Fatalf("runKubectl() error = %v, want a missing context error", err)
	}
}

func TestArtifactName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "TestSlurmChart_Install_Slurm", artifactName("TestSlurmChart/Install Slurm"))
	assert.Equal(t, "unnamed", artifactName("///"))
}

func TestUniqueStrings(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"slurm", "slinky", ""}, uniqueStrings([]string{"slurm", "slinky", "slurm", ""}))
}
