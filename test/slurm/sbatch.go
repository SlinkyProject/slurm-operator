// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// BatchJob describes a batch job whose script is a single shell command.
type BatchJob struct {
	Allocation
	// Command is the shell command the job runs.
	Command string
	// WorkDir is the job's working directory on the node it runs on.
	WorkDir string
	// Output is the stdout path on the node the job runs on. It may use the
	// sbatch filename patterns, such as %j for the job ID.
	Output string
}

func (j BatchJob) args() ([]string, error) {
	if j.Command == "" {
		return nil, errors.New("job command must not be empty")
	}
	allocation, err := j.Allocation.args()
	if err != nil {
		return nil, err
	}
	args := append([]string{"sbatch", "--parsable"}, allocation...)
	args = append(args, flags("--chdir", j.WorkDir, "--output", j.Output)...)
	return append(args, "--wrap="+j.Command), nil
}

// Submit submits a batch job and returns its ID without waiting for it to run.
func (c *Client) Submit(ctx context.Context, job BatchJob) (string, error) {
	args, err := job.args()
	if err != nil {
		return "", err
	}
	stdout, err := c.Exec(ctx, args...)
	if err != nil {
		return "", err
	}
	return parseJobID(stdout)
}

// parseJobID reads the "<id>" or "<id>;<cluster>" line sbatch --parsable prints.
func parseJobID(output string) (string, error) {
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return "", errors.New("sbatch printed no job ID")
	}
	jobID, _, _ := strings.Cut(fields[0], ";")
	if _, err := strconv.ParseUint(jobID, 10, 32); err != nil {
		return "", fmt.Errorf("sbatch printed an invalid job ID %q", output)
	}
	return jobID, nil
}
