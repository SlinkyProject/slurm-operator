// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import "slices"

// JobState is the base state of a Slurm job, without state flags such as
// COMPLETING or REQUEUED.
type JobState string

const (
	JobPending   JobState = "PENDING"
	JobRunning   JobState = "RUNNING"
	JobSuspended JobState = "SUSPENDED"
	JobCompleted JobState = "COMPLETED"
	JobCanceled  JobState = "CANCELLED" //nolint:misspell // Slurm's spelling
	JobFailed    JobState = "FAILED"
	JobTimeout   JobState = "TIMEOUT"
	JobNodeFail  JobState = "NODE_FAIL"
	JobOutOfMem  JobState = "OUT_OF_MEMORY"
	JobPreempted JobState = "PREEMPTED"
	JobBootFail  JobState = "BOOT_FAIL"
	JobDeadline  JobState = "DEADLINE"
)

var finishedJobStates = []JobState{
	JobCompleted,
	JobCanceled,
	JobFailed,
	JobTimeout,
	JobNodeFail,
	JobOutOfMem,
	JobPreempted,
	JobBootFail,
	JobDeadline,
}

// IsFinished reports whether the job can no longer change state.
func (s JobState) IsFinished() bool {
	return slices.Contains(finishedJobStates, s)
}

func (s JobState) isBase() bool {
	return s == JobPending || s == JobRunning || s == JobSuspended || s.IsFinished()
}
