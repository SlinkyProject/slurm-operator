// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtifactName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "TestSlurmChart_Install_Slurm", artifactName("TestSlurmChart/Install Slurm"))
	assert.Equal(t, "unnamed", artifactName("///"))
}

func TestUniqueStrings(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"slurm", "slinky", ""}, uniqueStrings([]string{"slurm", "slinky", "slurm", ""}))
}

func TestCommandOutputMatches(t *testing.T) {
	t.Parallel()

	assert.True(t, commandOutputMatches([]byte("idle\n"), "idle"))
	assert.True(t, commandOutputMatches([]byte("idle\nidle\n"), "idle"))
	assert.True(t, commandOutputMatches(nil, ""))
	assert.False(t, commandOutputMatches([]byte("idle\ndown\n"), "idle"))
	assert.False(t, commandOutputMatches(nil, "idle"))
}
