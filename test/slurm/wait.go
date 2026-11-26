// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// WaitForJobState polls the job until it reaches one of the wanted states. It
// fails early when the job finishes in a state that was not wanted.
func (c *Client) WaitForJobState(ctx context.Context, jobID string, timeout time.Duration, want ...JobState) (*Job, error) {
	if len(want) == 0 {
		return nil, errors.New("at least one wanted state is required")
	}

	var (
		job     *Job
		lastErr error
	)
	err := wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		current, err := c.Job(ctx, jobID)
		if err != nil {
			lastErr = err
			// Pod exec and scontrol fail transiently; retry until timeout.
			return false, nil //nolint:nilerr
		}
		job, lastErr = current, nil
		if slices.Contains(want, job.State) {
			return true, nil
		}
		if job.State.IsFinished() {
			return false, fmt.Errorf("job %s finished in state %s (reason %s, exit code %s), want one of %v",
				jobID, job.State, job.Reason, job.ExitCode, want)
		}
		return false, nil
	})
	if err != nil {
		if lastErr != nil {
			err = fmt.Errorf("%w; last error: %w", err, lastErr)
		}
		if job != nil {
			return job, fmt.Errorf("waiting for job %s to reach %v, last state %s (reason %s): %w",
				jobID, want, job.State, job.Reason, err)
		}
		return nil, fmt.Errorf("waiting for job %s to reach %v: %w", jobID, want, err)
	}
	return job, nil
}
