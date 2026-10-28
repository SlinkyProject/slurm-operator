// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"testing"

	"github.com/stretchr/testify/require"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
)

func TestFindJob(t *testing.T) {
	tests := []struct {
		name    string
		jobID   string
		output  string
		want    *Job
		wantErr bool
	}{
		{
			name:  "completed job",
			jobID: "7",
			output: `{"jobs": [{"job_id": 7, "name": "e2e", "account": "test", "job_state": ["COMPLETED"],
				"state_reason": "None", "exit_code": {"status": ["SUCCESS"], "return_code": {"set": true, "infinite": false, "number": 0}},
				"batch_host": "slinky-0", "standard_output": "/tmp/e2e-%j.out", "stdout_expanded": "/tmp/e2e-7.out"}],
				"meta": {}, "errors": [], "warnings": []}`,
			want: &Job{
				ID:        "7",
				Name:      "e2e",
				Account:   "test",
				State:     JobCompleted,
				Reason:    "None",
				ExitCode:  "0:0",
				BatchHost: "slinky-0",
				StdOut:    "/tmp/e2e-7.out",
			},
		},
		{
			name:   "completing job",
			jobID:  "8",
			output: `{"jobs": [{"job_id": 8, "name": "e2e", "job_state": ["COMPLETED", "COMPLETING"], "state_reason": "None"}]}`,
			want: &Job{
				ID:         "8",
				Name:       "e2e",
				State:      JobCompleted,
				StateFlags: []string{"COMPLETING"},
				Reason:     "None",
			},
		},
		{
			name:  "canceled job",
			jobID: "9",
			output: `{"jobs": [{"job_id": 9, "name": "e2e", "job_state": ["` + string(JobCanceled) + `"], "state_reason": "None",
				"exit_code": {"status": ["SIGNALED"], "return_code": {"set": true, "number": 0}, "signal": {"id": {"set": true, "number": 15}, "name": "SIGTERM"}},
				"batch_host": "slinky-0"}]}`,
			want: &Job{
				ID:        "9",
				Name:      "e2e",
				State:     JobCanceled,
				Reason:    "None",
				ExitCode:  "0:15",
				BatchHost: "slinky-0",
			},
		},
		{
			name:    "other job",
			jobID:   "7",
			output:  `{"jobs": [{"job_id": 70, "job_state": ["RUNNING"]}]}`,
			wantErr: true,
		},
		{
			name:    "Slurm error",
			jobID:   "7",
			output:  `{"jobs": [], "errors": [{"error": "Invalid job id specified", "error_number": 2017, "description": "", "source": "slurm_load_job()"}]}`,
			wantErr: true,
		},
		{
			name:    "not JSON",
			jobID:   "7",
			output:  "JobId=7 JobState=RUNNING",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &slurmapi.V0044OpenapiJobInfoResp{}
			err := decodeJSON(tt.output, resp)
			var got *Job
			if err == nil {
				got, err = findJob(tt.jobID, resp)
			}
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFindNode(t *testing.T) {
	tests := []struct {
		name     string
		node     string
		output   string
		want     *Node
		wantIdle bool
		wantErr  bool
	}{
		{
			name:   "idle dynamic node",
			node:   "slinky-0",
			output: `{"nodes": [{"name": "slinky-0", "address": "10.244.1.5", "state": ["IDLE", "DYNAMIC_NORM"], "topology": "topo-e2e:b0"}]}`,
			want: &Node{
				Name:     "slinky-0",
				Address:  "10.244.1.5",
				State:    []slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE, slurmapi.V0044NodeStateDYNAMICNORM},
				Topology: "topo-e2e:b0",
			},
			wantIdle: true,
		},
		{
			name:   "drained node",
			node:   "slinky-1",
			output: `{"nodes": [{"name": "slinky-1", "address": "10.244.1.6", "state": ["IDLE", "DRAIN"]}]}`,
			want: &Node{
				Name:    "slinky-1",
				Address: "10.244.1.6",
				State:   []slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE, slurmapi.V0044NodeStateDRAIN},
			},
		},
		{
			name:   "failing idle node",
			node:   "slinky-0",
			output: `{"nodes": [{"name": "slinky-0", "state": ["IDLE", "DYNAMIC_NORM", "FAIL"]}]}`,
			want: &Node{
				Name:  "slinky-0",
				State: []slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE, slurmapi.V0044NodeStateDYNAMICNORM, slurmapi.V0044NodeStateFAIL},
			},
		},
		{
			name:   "idle node in maintenance",
			node:   "slinky-0",
			output: `{"nodes": [{"name": "slinky-0", "state": ["IDLE", "MAINTENANCE"]}]}`,
			want: &Node{
				Name:  "slinky-0",
				State: []slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE, slurmapi.V0044NodeStateMAINTENANCE},
			},
		},
		{
			name:   "busy node",
			node:   "slinky-0",
			output: `{"nodes": [{"name": "slinky-0", "state": ["MIXED"]}]}`,
			want: &Node{
				Name:  "slinky-0",
				State: []slurmapi.V0044NodeState{slurmapi.V0044NodeStateMIXED},
			},
		},
		{
			name:    "no node",
			node:    "slinky-0",
			output:  `{"nodes": []}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &slurmapi.V0044OpenapiNodesResp{}
			require.NoError(t, decodeJSON(tt.output, resp))
			got, err := findNode(tt.node, resp)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.wantIdle, got.IsIdle())
		})
	}
}
