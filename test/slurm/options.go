// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"fmt"
	"strconv"
)

// Allocation holds the resource options that sbatch and srun share.
type Allocation struct {
	// Name is the job name, which CancelJobsByName matches.
	Name string
	// Account is the Slurm account the job is charged to.
	Account string
	// Nodes is the number of nodes to allocate. Zero leaves it to Slurm.
	Nodes int
	// NodeList pins the job to the given Slurm nodes.
	NodeList string
	// TimeLimit uses the Slurm --time format, for example "0:15".
	TimeLimit string
}

func (a Allocation) args() ([]string, error) {
	if a.Nodes < 0 {
		return nil, fmt.Errorf("node count must not be negative, got %d", a.Nodes)
	}
	var nodes string
	if a.Nodes > 0 {
		nodes = strconv.Itoa(a.Nodes)
	}
	return flags(
		"--job-name", a.Name,
		"--account", a.Account,
		"--nodes", nodes,
		"--nodelist", a.NodeList,
		"--time", a.TimeLimit,
	), nil
}

// flags turns name/value pairs into "--name=value" arguments, skipping empty
// values.
func flags(pairs ...string) []string {
	var args []string
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != "" {
			args = append(args, pairs[i]+"="+pairs[i+1])
		}
	}
	return args
}
