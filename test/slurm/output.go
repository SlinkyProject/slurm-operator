// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"context"
	"fmt"
)

// JobOutput returns the contents of the job's stdout file, read from the
// worker pod of the node that ran the batch script. Stderr is included only
// when the job did not redirect it to a separate file.
func (c *Client) JobOutput(ctx context.Context, jobID string) (string, error) {
	job, err := c.Job(ctx, jobID)
	if err != nil {
		return "", err
	}
	if job.BatchHost == "" {
		return "", fmt.Errorf("job %s has no batch host yet (state %s)", jobID, job.State)
	}
	if job.StdOut == "" {
		return "", fmt.Errorf("job %s has no stdout path", jobID)
	}

	pod, err := c.NodePod(ctx, job.BatchHost)
	if err != nil {
		return "", fmt.Errorf("job %s: %w", jobID, err)
	}
	return c.InPod(pod).Exec(ctx, "cat", job.StdOut)
}
