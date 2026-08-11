// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"errors"
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
func WaitForCommand(ctx context.Context, t *testing.T, command string, args []string, wants string, cleanup_command string, cleanup_args []string, timeout time.Duration, retryDelay time.Duration) {
	t.Helper()

	var output []byte
	err := wait.For(func(waitCtx context.Context) (bool, error) {
		if cleanup_command != "" && len(cleanup_args) > 0 {
			cleanup_cmd := exec.CommandContext(waitCtx, cleanup_command, cleanup_args...)

			_, _ = cleanup_cmd.Output() //nolint:errcheck
		}

		cmd := exec.CommandContext(waitCtx, command, args...)
		var commandErr error
		output, commandErr = cmd.Output()
		return commandErr == nil && (wants == "" || strings.TrimSpace(string(output)) == wants), nil
	},
		wait.WithContext(ctx),
		wait.WithTimeout(timeout),
		wait.WithInterval(retryDelay),
		wait.WithImmediate(),
	)

	require.NoError(t, err, "timed out waiting for %v %v; last output: %q", command, args, strings.TrimSpace(string(output)))
}

// GetSlurmNodeInfo uses scontrol to get details on a Slurm node
func GetSlurmNodeInfo(nodeName string) (map[string]string, error) {
	command := "kubectl"
	args := []string{
		"exec", "-n", SlurmNamespace, "slurm-controller-0", "--",
		"scontrol", "show", "node", nodeName,
	}

	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.New("failed executing command")
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
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: deploymentNamespace,
		},
	}
	err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, 1))
	require.NoError(t, err, "failed waiting for the %s deployment to reach a ready state", deploymentName)

	return ctx
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
