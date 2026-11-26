// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"context"
	"errors"
)

// CancelJob cancels a job.
func (c *Client) CancelJob(ctx context.Context, jobID string) error {
	if jobID == "" {
		return errors.New("job ID must not be empty")
	}
	_, err := c.Exec(ctx, "scancel", jobID)
	return err
}

// CancelJobsByName cancels every job with the given name.
func (c *Client) CancelJobsByName(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("job name must not be empty")
	}
	_, err := c.Exec(ctx, "scancel", "--name="+name)
	return err
}

// CancelJobsByUser cancels every job the given user owns.
func (c *Client) CancelJobsByUser(ctx context.Context, user string) error {
	if user == "" {
		return errors.New("user name must not be empty")
	}
	_, err := c.Exec(ctx, "scancel", "--user="+user)
	return err
}
