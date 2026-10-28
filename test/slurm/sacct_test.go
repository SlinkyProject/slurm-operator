// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"testing"

	"github.com/stretchr/testify/require"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
)

func TestFindAccountingRecord(t *testing.T) {
	tests := []struct {
		name    string
		jobID   string
		output  string
		want    *AccountingRecord
		wantErr bool
	}{
		{
			name:  "completed job",
			jobID: "7",
			output: `{"jobs": [{"job_id": 7, "name": "e2e", "account": "test", "user": "slurm",
				"state": {"current": ["COMPLETED"], "reason": "None"},
				"exit_code": {"status": ["SUCCESS"], "return_code": {"set": true, "number": 0}}}]}`,
			want: &AccountingRecord{
				Name:     "e2e",
				Account:  "test",
				User:     "slurm",
				State:    JobCompleted,
				ExitCode: "0:0",
			},
		},
		{
			name:  "canceled job",
			jobID: "9",
			output: `{"jobs": [{"job_id": 9, "name": "e2e", "account": "test", "user": "slurm",
				"state": {"current": ["` + string(JobCanceled) + `"], "reason": "None"},
				"exit_code": {"status": ["SIGNALED"], "return_code": {"set": true, "number": 0}, "signal": {"id": {"set": true, "number": 15}}}}]}`,
			want: &AccountingRecord{
				Name:     "e2e",
				Account:  "test",
				User:     "slurm",
				State:    JobCanceled,
				ExitCode: "0:15",
			},
		},
		{
			name:    "no record yet",
			jobID:   "7",
			output:  `{"jobs": []}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &slurmapi.V0044OpenapiSlurmdbdJobsResp{}
			require.NoError(t, decodeJSON(tt.output, resp))
			got, err := findAccountingRecord(tt.jobID, resp)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
