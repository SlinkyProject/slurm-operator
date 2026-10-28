// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJobStateIsFinished(t *testing.T) {
	for _, state := range []JobState{JobPending, JobRunning, JobSuspended} {
		require.False(t, state.IsFinished(), "state %s", state)
	}
	for _, state := range []JobState{JobCompleted, JobCanceled, JobFailed, JobTimeout} {
		require.True(t, state.IsFinished(), "state %s", state)
	}
}

func TestSplitJobState(t *testing.T) {
	base, flags := splitJobState([]string{"COMPLETING", "COMPLETED"})
	require.Equal(t, JobCompleted, base)
	require.Equal(t, []string{"COMPLETING"}, flags)

	base, flags = splitJobState([]string{"PENDING"})
	require.Equal(t, JobPending, base)
	require.Empty(t, flags)
}
