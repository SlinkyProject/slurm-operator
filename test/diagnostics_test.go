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
