// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatchJobArgs(t *testing.T) {
	tests := []struct {
		name    string
		job     BatchJob
		want    []string
		wantErr bool
	}{
		{
			name: "command only",
			job:  BatchJob{Command: "hostname"},
			want: []string{"sbatch", "--parsable", "--wrap=hostname"},
		},
		{
			name: "all options",
			job: BatchJob{
				Allocation: Allocation{
					Name:      "e2e",
					Account:   "test",
					Nodes:     2,
					NodeList:  "slinky-[0-1]",
					TimeLimit: "0:15",
				},
				Command: "sleep 300",
				WorkDir: "/tmp",
				Output:  "/tmp/e2e-%j.out",
			},
			want: []string{
				"sbatch", "--parsable",
				"--job-name=e2e",
				"--account=test",
				"--nodes=2",
				"--nodelist=slinky-[0-1]",
				"--time=0:15",
				"--chdir=/tmp",
				"--output=/tmp/e2e-%j.out",
				"--wrap=sleep 300",
			},
		},
		{
			name:    "empty command",
			job:     BatchJob{Allocation: Allocation{Name: "e2e"}},
			wantErr: true,
		},
		{
			name:    "negative node count",
			job:     BatchJob{Allocation: Allocation{Nodes: -1}, Command: "hostname"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.job.args()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseJobID(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{name: "job ID", output: "42\n", want: "42"},
		{name: "job ID with cluster", output: "42;slurm\n", want: "42"},
		{name: "empty", output: "\n", wantErr: true},
		{name: "not a number", output: "Submitted batch job 42\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJobID(tt.output)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
