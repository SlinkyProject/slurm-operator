// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"k8s.io/utils/ptr"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
)

// DataParser is the Slurm data_parser plugin version that the client asks
// for with --json. It matches the slurm-client API package that decodes the
// output, so bump both together.
const DataParser = "v0.0.44"

// execJSON runs a Slurm command with --json and decodes its output into resp.
func (c *Client) execJSON(ctx context.Context, resp any, command string, args ...string) error {
	stdout, err := c.Exec(ctx, append([]string{command, "--json=" + DataParser}, args...)...)
	if err != nil {
		return err
	}
	return decodeJSON(stdout, resp)
}

// decodeJSON decodes the JSON output of a Slurm command into resp. It fails
// when the output reports errors.
func decodeJSON(output string, resp any) error {
	var status struct {
		Errors slurmapi.V0044OpenapiErrors `json:"errors"`
	}
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return fmt.Errorf("decode Slurm JSON output: %w", err)
	}
	if len(status.Errors) > 0 {
		messages := make([]string, 0, len(status.Errors))
		for _, apiErr := range status.Errors {
			messages = append(messages, strings.TrimSpace(ptr.Deref(apiErr.Error, "")+": "+ptr.Deref(apiErr.Description, "")))
		}
		return errors.New("Slurm reported errors: " + strings.Join(messages, "; ")) //nolint:staticcheck // Slurm is a proper noun
	}
	if err := json.Unmarshal([]byte(output), resp); err != nil {
		return fmt.Errorf("decode Slurm JSON output: %w", err)
	}
	return nil
}

// exitCode formats an exit code as "<return code>:<signal>", as scontrol and
// sacct print it.
func exitCode(code *slurmapi.V0044ProcessExitCodeVerbose) string {
	if code == nil {
		return ""
	}
	var returnCode, signal int32
	if code.ReturnCode != nil {
		returnCode = ptr.Deref(code.ReturnCode.Number, 0)
	}
	if code.Signal != nil && code.Signal.Id != nil {
		signal = ptr.Deref(code.Signal.Id.Number, 0)
	}
	return fmt.Sprintf("%d:%d", returnCode, signal)
}

// splitJobState separates the base state of a job from its state flags.
func splitJobState[T ~string](states []T) (JobState, []string) {
	var (
		base  JobState
		flags []string
	)
	for _, state := range states {
		if base == "" && JobState(state).isBase() {
			base = JobState(state)
			continue
		}
		flags = append(flags, string(state))
	}
	return base, flags
}
