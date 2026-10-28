// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"
)

// Step describes a command that srun runs in a new allocation.
type Step struct {
	Allocation
	// Immediate makes srun fail when the allocation is not granted within
	// this duration, rounded up to whole seconds. Zero waits indefinitely.
	Immediate time.Duration
	// Command is the program and arguments each task runs.
	Command []string
}

func (s Step) args() ([]string, error) {
	if len(s.Command) == 0 {
		return nil, errors.New("step command must not be empty")
	}
	if s.Immediate < 0 {
		return nil, errors.New("immediate timeout must not be negative")
	}
	allocation, err := s.Allocation.args()
	if err != nil {
		return nil, err
	}
	args := append([]string{"srun", "--quiet", "--kill-on-bad-exit=1"}, allocation...)
	if s.Immediate > 0 {
		args = append(args, "--immediate="+strconv.Itoa(int(math.Ceil(s.Immediate.Seconds()))))
	}
	return append(args, s.Command...), nil
}

// Run runs the step to completion and returns the stdout of its tasks.
func (c *Client) Run(ctx context.Context, step Step) (string, error) {
	args, err := step.args()
	if err != nil {
		return "", err
	}
	return c.Exec(ctx, args...)
}
