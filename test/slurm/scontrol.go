// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"k8s.io/utils/ptr"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
)

// Job is the subset of scontrol job details that tests assert on.
type Job struct {
	ID      string
	Name    string
	Account string
	State   JobState
	// StateFlags holds the job's state flags, such as COMPLETING.
	StateFlags []string
	Reason     string
	ExitCode   string
	BatchHost  string
	// StdOut is a path on BatchHost, with filename patterns already
	// expanded by Slurm.
	StdOut string
}

// Node is the subset of scontrol node details that tests assert on.
type Node struct {
	Name    string
	Address string
	// State holds the base state and the state flags, for example IDLE and
	// DRAIN.
	State    []slurmapi.V0044NodeState
	Topology string
}

// idleNodeStates are the states an idle node may report. NodeSet workers
// register as dynamic nodes, so they carry DYNAMIC_NORM next to IDLE.
var idleNodeStates = []slurmapi.V0044NodeState{
	slurmapi.V0044NodeStateIDLE,
	slurmapi.V0044NodeStateDYNAMICNORM,
}

// IsIdle reports whether the node is idle with no other state flag, so a node
// that is also failing, draining, down, rebooting or completing is not idle.
func (n *Node) IsIdle() bool {
	if !slices.Contains(n.State, slurmapi.V0044NodeStateIDLE) {
		return false
	}
	for _, state := range n.State {
		if !slices.Contains(idleNodeStates, state) {
			return false
		}
	}
	return true
}

// PingController checks that slurmctld responds.
func (c *Client) PingController(ctx context.Context) error {
	_, err := c.Exec(ctx, "scontrol", "ping")
	return err
}

// Job returns the current details of a job. Slurm forgets finished jobs after
// MinJobAge, so Job fails for jobs that ended long ago.
func (c *Client) Job(ctx context.Context, jobID string) (*Job, error) {
	resp := &slurmapi.V0044OpenapiJobInfoResp{}
	if err := c.execJSON(ctx, resp, "scontrol", "show", "job", jobID); err != nil {
		return nil, err
	}
	return findJob(jobID, resp)
}

// Node returns the current details of a Slurm node.
func (c *Client) Node(ctx context.Context, name string) (*Node, error) {
	resp := &slurmapi.V0044OpenapiNodesResp{}
	if err := c.execJSON(ctx, resp, "scontrol", "show", "node", name); err != nil {
		return nil, err
	}
	return findNode(name, resp)
}

func findJob(jobID string, resp *slurmapi.V0044OpenapiJobInfoResp) (*Job, error) {
	for _, info := range resp.Jobs {
		if strconv.Itoa(int(ptr.Deref(info.JobId, 0))) != jobID {
			continue
		}
		state, flags := splitJobState(ptr.Deref(info.JobState, nil))
		return &Job{
			ID:         jobID,
			Name:       ptr.Deref(info.Name, ""),
			Account:    ptr.Deref(info.Account, ""),
			State:      state,
			StateFlags: flags,
			Reason:     ptr.Deref(info.StateReason, ""),
			ExitCode:   exitCode(info.ExitCode),
			BatchHost:  ptr.Deref(info.BatchHost, ""),
			StdOut:     ptr.Deref(info.StdoutExpanded, ""),
		}, nil
	}
	return nil, fmt.Errorf("scontrol reports no job %s", jobID)
}

func findNode(name string, resp *slurmapi.V0044OpenapiNodesResp) (*Node, error) {
	for _, node := range resp.Nodes {
		if ptr.Deref(node.Name, "") != name {
			continue
		}
		return &Node{
			Name:     name,
			Address:  ptr.Deref(node.Address, ""),
			State:    ptr.Deref(node.State, nil),
			Topology: ptr.Deref(node.Topology, ""),
		}, nil
	}
	return nil, fmt.Errorf("scontrol reports no node %s", name)
}
