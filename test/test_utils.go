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
)

// getBasePath returns the fully qualified path of the slurm-operator repo within the context in which `go test` is called
func GetBasePath() string {
	_, b, _, _ := runtime.Caller(0)
	fullpath := filepath.Dir(b)
	path, _ := strings.CutSuffix(fullpath, "test")

	return path
}

func RetryCommand(ctx context.Context, t *testing.T, command string, args []string, wants string, cleanup_command string, cleanup_args []string, retries int, retryDelay time.Duration) context.Context {
	for retry := range retries {

		if cleanup_command != "" && len(cleanup_args) > 0 {
			cleanup_cmd := exec.Command(cleanup_command, cleanup_args...)

			_, _ = cleanup_cmd.Output() //nolint:errcheck
		}

		cmd := exec.Command(command, args...)

		output, err := cmd.Output()
		if err == nil && (wants == "" || strings.TrimSpace(string(output)) == wants) {
			return ctx
		}

		if retry == retries-retry {
			require.NoError(t, err, "failed running %v %v", command, args)
			require.Equal(t, wants, strings.TrimSpace(string(output)))

			return ctx
		}

		time.Sleep(retryDelay)
	}

	return ctx
}

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
