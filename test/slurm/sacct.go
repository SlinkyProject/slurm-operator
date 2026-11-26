// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"context"
	"fmt"
	"strconv"

	"k8s.io/utils/ptr"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
)

// AccountingRecord is the job allocation as recorded by slurmdbd.
type AccountingRecord struct {
	Name    string
	Account string
	User    string
	State   JobState
	// StateFlags holds the job's state flags, such as REQUEUED.
	StateFlags []string
	ExitCode   string
}

// JobAccounting returns the job's allocation record from sacct. It requires
// accounting, and slurmctld forwards records to slurmdbd asynchronously, so
// callers should retry until the record appears.
func (c *Client) JobAccounting(ctx context.Context, jobID string) (*AccountingRecord, error) {
	resp := &slurmapi.V0044OpenapiSlurmdbdJobsResp{}
	if err := c.execJSON(ctx, resp, "sacct", "--jobs="+jobID); err != nil {
		return nil, err
	}
	return findAccountingRecord(jobID, resp)
}

func findAccountingRecord(jobID string, resp *slurmapi.V0044OpenapiSlurmdbdJobsResp) (*AccountingRecord, error) {
	for _, job := range resp.Jobs {
		if strconv.Itoa(int(ptr.Deref(job.JobId, 0))) != jobID {
			continue
		}
		record := &AccountingRecord{
			Name:     ptr.Deref(job.Name, ""),
			Account:  ptr.Deref(job.Account, ""),
			User:     ptr.Deref(job.User, ""),
			ExitCode: exitCode(job.ExitCode),
		}
		if job.State != nil {
			record.State, record.StateFlags = splitJobState(ptr.Deref(job.State.Current, nil))
		}
		return record, nil
	}
	return nil, fmt.Errorf("sacct has no record for job %s", jobID)
}
