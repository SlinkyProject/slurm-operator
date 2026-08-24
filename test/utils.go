// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

// getBasePath returns the fully qualified path of the slurm-operator repo within the context in which `go test` is called
func GetBasePath() string {
	_, b, _, _ := runtime.Caller(0)
	fullpath := filepath.Dir(b)
	path, _ := strings.CutSuffix(fullpath, "test")

	return path
}

// WaitForCommand executes the provided command until it succeeds with the expected output.
func WaitForCommand(ctx context.Context, t *testing.T, command string, args []string, wants string, cleanupCommand string, cleanupArgs []string, timeout time.Duration, retryDelay time.Duration) {
	t.Helper()

	var output []byte
	var commandErr error
	attempts := 0
	started := time.Now()
	err := wait.For(func(waitCtx context.Context) (bool, error) {
		attempts++
		if cleanupCommand != "" && len(cleanupArgs) > 0 {
			cleanupCmd := exec.CommandContext(waitCtx, cleanupCommand, cleanupArgs...)

			_, _ = cleanupCmd.CombinedOutput() //nolint:errcheck
		}

		cmd := exec.CommandContext(waitCtx, command, args...)
		output, commandErr = cmd.CombinedOutput()
		return commandErr == nil && (wants == "" || strings.TrimSpace(string(output)) == wants), nil
	},
		wait.WithContext(ctx),
		wait.WithTimeout(timeout),
		wait.WithInterval(retryDelay),
		wait.WithImmediate(),
	)

	if err != nil {
		expected := fmt.Sprintf("output %q", wants)
		if wants == "" {
			expected = "a successful exit"
		}
		t.Fatalf(
			"command did not reach %s after %s (%d attempts): %s %s\nwait error: %v\nlast command error: %v\nlast combined output: %q",
			expected,
			time.Since(started).Round(time.Millisecond),
			attempts,
			command,
			strings.Join(args, " "),
			err,
			commandErr,
			strings.TrimSpace(string(output)),
		)
	}
}

// GetSlurmNodeInfo uses scontrol to get details on a Slurm node
func GetSlurmNodeInfo(nodeName string) (map[string]string, error) {
	command := "kubectl"
	args := []string{
		"exec", "-n", SlurmNamespace, "slurm-controller-0", "--",
		"scontrol", "show", "node", nodeName,
	}

	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed executing %s %s: %w; combined output: %q", command, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}

	out_map := StringToMap(string(output))
	return out_map, nil
}

// StringToMap converts the provided string to a map.
// Assumes that key-value pairs are separated by spaces,
// and that keys and values are separated by an equals sign
func StringToMap(input string) map[string]string {
	out_array := strings.Split(string(input), " ")
	out_map := make(map[string]string)

	for _, val := range out_array {
		object := strings.Split(val, "=")
		if len(object) == 2 {
			key := object[0]
			value := object[1]

			out_map[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}

	return out_map
}

// CheckDeploymentStatus waits for the specified deployment to have the desired number of ReadyReplicas, then returns
func CheckDeploymentStatus(ctx context.Context, t *testing.T, config *envconf.Config, deploymentName string, deploymentNamespace string) context.Context {
	t.Helper()

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: deploymentNamespace,
		},
	}
	err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, 1))
	require.NoError(
		t,
		err,
		"failed waiting for deployment %s/%s to reach one ready replica; observed status: %s",
		deploymentNamespace,
		deploymentName,
		StatusJSON(deployment.Status),
	)

	return ctx
}

// StatusJSON renders Kubernetes status structs compactly for failure messages.
func StatusJSON(status any) string {
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Sprintf("<failed to marshal status: %v>", err)
	}
	return string(data)
}

// DoUninstallHelmChart uses the Helm API to uninstall the specified Helm chart in the specified namespace
func DoUninstallHelmChart(ctx context.Context, t *testing.T, config *envconf.Config, chartName string, chartNamespace string) context.Context {
	manager := helm.New(config.KubeconfigFile())

	err := manager.RunUninstall(
		helm.WithName(chartName),
		helm.WithNamespace(chartNamespace),
		helm.WithWait(),
		helm.WithTimeout("5m"),
	)
	require.NoError(t, err, "failed to invoke helm uninstall %s due to an error", chartName)

	return ctx
}
