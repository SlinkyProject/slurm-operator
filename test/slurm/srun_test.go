// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStepArgs(t *testing.T) {
	tests := []struct {
		name    string
		step    Step
		want    []string
		wantErr bool
	}{
		{
			name: "command only",
			step: Step{Command: []string{"hostname"}},
			want: []string{"srun", "--quiet", "--kill-on-bad-exit=1", "hostname"},
		},
		{
			name: "all options",
			step: Step{
				Allocation: Allocation{
					Name:      "e2e",
					Nodes:     2,
					NodeList:  "slinky-[0-1]",
					TimeLimit: "0:15",
				},
				Immediate: 9500 * time.Millisecond,
				Command:   []string{"sh", "-c", "echo $SLURM_NODEID"},
			},
			want: []string{
				"srun", "--quiet", "--kill-on-bad-exit=1",
				"--job-name=e2e",
				"--nodes=2",
				"--nodelist=slinky-[0-1]",
				"--time=0:15",
				"--immediate=10",
				"sh", "-c", "echo $SLURM_NODEID",
			},
		},
		{
			name:    "empty command",
			step:    Step{Allocation: Allocation{Name: "e2e"}},
			wantErr: true,
		},
		{
			name:    "negative immediate timeout",
			step:    Step{Immediate: -time.Second, Command: []string{"hostname"}},
			wantErr: true,
		},
		{
			name:    "negative node count",
			step:    Step{Allocation: Allocation{Nodes: -1}, Command: []string{"hostname"}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.step.args()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
