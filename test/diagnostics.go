// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
)

const (
	defaultE2EArtifactsDir = "e2e-artifacts"
	diagnosticTimeout      = 20 * time.Second
)

// CaptureFailureDiagnostics writes a compact snapshot of the cluster while the
// resources involved in a failed E2E scenario still exist. Diagnostic failures
// are recorded in the artifact files but never obscure the original test error.
func CaptureFailureDiagnostics(t *testing.T, failure string, namespaces ...string) string {
	t.Helper()

	artifactRoot := os.Getenv("E2E_ARTIFACTS_DIR")
	if artifactRoot == "" {
		artifactRoot = defaultE2EArtifactsDir
		if Basepath != "" {
			artifactRoot = filepath.Join(Basepath, artifactRoot)
		}
	}

	dir := filepath.Join(artifactRoot, "failures", artifactName(t.Name()))
	// E2E_ARTIFACTS_DIR intentionally allows the CI job to choose its artifact root.
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec
		t.Logf("failed to create E2E diagnostic directory %q: %v", dir, err)
		return ""
	}

	summary := fmt.Sprintf(
		"captured: %s\ntest: %s\nfailure: %s\n",
		time.Now().UTC().Format(time.RFC3339Nano),
		t.Name(),
		failure,
	)
	writeDiagnosticFile(t, filepath.Join(dir, "summary.txt"), []byte(summary))

	captureKubectl(t, dir, "cluster-nodes.txt", "get", "nodes", "-o", "wide")
	captureKubectl(t, dir, "cluster-pods.txt", "get", "pods", "--all-namespaces", "-o", "wide")

	for _, namespace := range uniqueStrings(namespaces) {
		if namespace == "" {
			continue
		}

		namespaceDir := filepath.Join(dir, artifactName(namespace))
		if err := os.MkdirAll(namespaceDir, 0o755); err != nil { //nolint:gosec // namespace is reduced to a safe artifact name
			t.Logf("failed to create namespace diagnostic directory %q: %v", namespaceDir, err)
			continue
		}

		captureKubectl(t, namespaceDir, "workloads.txt",
			"get", "pods,deployments,statefulsets,daemonsets,services,persistentvolumeclaims",
			"--namespace", namespace, "-o", "wide")
		captureKubectl(t, namespaceDir, "events.txt",
			"get", "events", "--namespace", namespace, "--sort-by=.metadata.creationTimestamp")
		captureKubectl(t, namespaceDir, "pods-describe.txt",
			"describe", "pods", "--namespace", namespace)
		captureKubectl(t, namespaceDir, "slinky-resources.yaml",
			"get", strings.Join([]string{
				"controllers.slinky.slurm.net",
				"nodesets.slinky.slurm.net",
				"loginsets.slinky.slurm.net",
				"accountings.slinky.slurm.net",
				"restapis.slinky.slurm.net",
			}, ","), "--namespace", namespace, "-o", "yaml")
		if namespace != SlinkyNamespace {
			captureKubectl(t, namespaceDir, "slurm-nodes.txt",
				"exec", "--namespace", namespace, "slurm-controller-0", "--",
				"scontrol", "show", "nodes", "--details")
			captureKubectl(t, namespaceDir, "sinfo.txt",
				"exec", "--namespace", namespace, "slurm-controller-0", "--",
				"sinfo", "--Node", "--long")
		}

		capturePodLogs(t, namespaceDir, namespace)
	}

	t.Logf("E2E failure diagnostics written to %s", dir)
	return dir
}

func capturePodLogs(t *testing.T, dir, namespace string) {
	t.Helper()

	output, err := runKubectl("get", "pods", "--namespace", namespace, "-o", "name")
	if err != nil {
		writeDiagnosticFile(t, filepath.Join(dir, "pod-logs-error.txt"), diagnosticOutput(err, output))
		return
	}

	for _, pod := range strings.Fields(string(output)) {
		name := artifactName(strings.TrimPrefix(pod, "pod/"))
		captureKubectl(t, dir, name+".log",
			"logs", "--namespace", namespace, pod, "--all-containers=true",
			"--prefix=true", "--timestamps=true", "--tail=500")
		captureKubectl(t, dir, name+"-previous.log",
			"logs", "--namespace", namespace, pod, "--all-containers=true",
			"--prefix=true", "--timestamps=true", "--tail=500", "--previous")
	}
}

func captureKubectl(t *testing.T, dir, filename string, args ...string) {
	t.Helper()

	output, err := runKubectl(args...)
	header := []byte("$ kubectl " + strings.Join(args, " ") + "\n")
	writeDiagnosticFile(t, filepath.Join(dir, filename), append(header, diagnosticOutput(err, output)...))
}

func runKubectl(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), diagnosticTimeout)
	defer cancel()

	return exec.CommandContext(ctx, "kubectl", args...).CombinedOutput()
}

func diagnosticOutput(err error, output []byte) []byte {
	if err == nil {
		return output
	}

	return []byte(fmt.Sprintf("error: %v\n%s", err, output))
}

func writeDiagnosticFile(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0o600); err != nil { //nolint:gosec // path is rooted in the configured artifact directory
		t.Logf("failed to write E2E diagnostic file %q: %v", path, err)
	}
}

func artifactName(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, value)

	value = strings.Trim(value, "._-")
	if value == "" {
		return "unnamed"
	}
	return value
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}
