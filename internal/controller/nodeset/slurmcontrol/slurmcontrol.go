// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmcontrol

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/puttsk/hostlist"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	slurmobject "github.com/SlinkyProject/slurm-client/pkg/object"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podinfo"
	"github.com/SlinkyProject/slurm-operator/internal/utils/timestore"
	slurmconditions "github.com/SlinkyProject/slurm-operator/pkg/conditions"
)

type SlurmControlInterface interface {
	// RefreshNodeCache forces the Node cache to be refreshed
	RefreshNodeCache(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) error
	// UpdateNodeWithPodInfo handles updating the Node with its pod info
	UpdateNodeWithPodInfo(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error
	// MakeNodeDrain handles adding the DRAIN state to the slurm node.
	MakeNodeDrain(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, reason string) error
	// MakeNodeUndrain handles removing the DRAIN state from the slurm node.
	MakeNodeUndrain(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, reason string) error
	// IsNodeDrain checks if the slurm node has the DRAIN state.
	IsNodeDrain(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error)
	// IsNodeDrained checks if the slurm node is drained.
	IsNodeDrained(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error)
	// IsNodeDownForUnresponsive checks if the slurm node is unresponsive
	IsNodeDownForUnresponsive(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error)
	// IsNodeReasonOurs reports if the node reason was set by the operator.
	IsNodeReasonOurs(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error)
	// CalculateNodeStatus returns the current state of the registered slurm nodes.
	CalculateNodeStatus(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) (SlurmNodeStatus, error)
	// GetNodeDeadlines returns a map of node to its deadline time.Time calculated from running jobs.
	GetNodeDeadlines(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) (*timestore.TimeStore, error)
}

// realSlurmControl is the default implementation of SlurmControlInterface.
type realSlurmControl struct {
	clientMap *clientmap.ClientMap
}

// RefreshNodeCache implements SlurmControlInterface.
func (r *realSlurmControl) RefreshNodeCache(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) error {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do RefreshNodeCache()")
		return nil
	}

	nodeList := &slurmtypes.V0044NodeList{}
	opts := &slurmclient.ListOptions{RefreshCache: true}
	if err := slurmClient.List(ctx, nodeList, opts); err != nil {
		return err
	}

	return nil
}

// GetNodeNames implements SlurmControlInterface.
func (r *realSlurmControl) GetNodeNames(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) ([]string, error) {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do GetNodeNames()")
		return nil, nil
	}

	nodeList := &slurmtypes.V0044NodeList{}
	if err := slurmClient.List(ctx, nodeList); err != nil {
		return nil, err
	}

	podNodeNameSet := set.New[string]()
	for _, pod := range pods {
		podNodeName := nodesetutils.GetNodeName(pod)
		podNodeNameSet.Insert(podNodeName)
	}

	nodeNames := []string{}
	for _, node := range nodeList.Items {
		nodeName := ptr.Deref(node.Name, "")
		if !podNodeNameSet.Has(nodeName) {
			continue
		}
		nodeNames = append(nodeNames, nodeName)
	}

	return nodeNames, nil
}

// UpdateNodeWithPodInfo implements SlurmControlInterface.
func (r *realSlurmControl) UpdateNodeWithPodInfo(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do UpdateNodeWithPodInfo()",
			"pod", klog.KObj(pod))
		return nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	podInfo := podinfo.PodInfo{
		Namespace: pod.GetNamespace(),
		PodName:   pod.GetName(),
		Node:      pod.Spec.NodeName,
	}
	podInfoOld := &podinfo.PodInfo{}
	_ = podinfo.ParseIntoPodInfo(slurmNode.Comment, podInfoOld)

	if podInfoOld.Equal(podInfo) {
		logger.V(3).Info("Node already contains podInfo, skipping update request",
			"node", slurmNode.GetKey(), "podInfo", podInfo)
		return nil
	}

	logger.Info("Update Slurm Node with Kubernetes Pod info",
		"Node", slurmNode.Name, "podInfo", podInfo)
	req := slurmapi.V0044UpdateNodeMsg{
		Comment: ptr.To(podInfo.ToString()),
	}
	if err := slurmClient.Update(ctx, slurmNode, req); err != nil {
		if !tolerateError(err) {
			return err
		}
	}

	if podInfoOld.Node != "" {
		logger.Info("Update Slurm Node state due to Kubernetes node migration", "Node", slurmNode.Name)
		req := slurmapi.V0044UpdateNodeMsg{
			State: ptr.To([]slurmapi.V0044UpdateNodeMsgState{slurmapi.V0044UpdateNodeMsgStateIDLE}),
		}
		if err := slurmClient.Update(ctx, slurmNode, req); err != nil {
			if tolerateError(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

const nodeReasonPrefix = "slurm-operator:"

// MakeNodeDrain implements SlurmControlInterface.
func (r *realSlurmControl) MakeNodeDrain(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, reason string) error {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do MakeNodeDrain()",
			"pod", klog.KObj(pod))
		return nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	// If the reason is not empty, prefix it with nodeReasonPrefix
	prefixedReason := ""
	if reason != "" {
		prefixedReason = nodeReasonPrefix + " " + reason
	}

	// If Slurm node is already drained and the reasons match, no need to drain it again
	nodeReason := ptr.Deref(slurmNode.Reason, "")
	if slurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateDRAIN) && nodeReason == prefixedReason {
		logger.V(1).Info("Node is already drained, skipping drain request",
			"node", slurmNode.GetKey(), "nodeState", slurmNode.State, "nodeReason", nodeReason)
		return nil
	}

	logger.V(1).Info("make slurm node drain",
		"pod", klog.KObj(pod))
	req := slurmapi.V0044UpdateNodeMsg{
		State:  ptr.To([]slurmapi.V0044UpdateNodeMsgState{slurmapi.V0044UpdateNodeMsgStateDRAIN}),
		Reason: ptr.To(prefixedReason),
	}
	if err := slurmClient.Update(ctx, slurmNode, req); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	return nil
}

// MakeNodeUndrain implements SlurmControlInterface.
func (r *realSlurmControl) MakeNodeUndrain(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, reason string) error {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do MakeNodeUndrain()",
			"pod", klog.KObj(pod))
		return nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	if !slurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateDRAIN) ||
		slurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateUNDRAIN) {
		logger.V(1).Info("Node is already undrained, skipping undrain request",
			"node", slurmNode.GetKey(), "nodeState", slurmNode.State)
		return nil
	}

	// If the reason is not empty, prefix it with nodeReasonPrefix
	prefixedReason := ""
	if reason != "" {
		prefixedReason = nodeReasonPrefix + " " + reason
	}

	logger.V(1).Info("make slurm node undrain",
		"pod", klog.KObj(pod))
	req := slurmapi.V0044UpdateNodeMsg{
		State:  ptr.To([]slurmapi.V0044UpdateNodeMsgState{slurmapi.V0044UpdateNodeMsgStateUNDRAIN}),
		Reason: ptr.To(prefixedReason),
	}
	if err := slurmClient.Update(ctx, slurmNode, req); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	return nil
}

// IsNodeDrain implements SlurmControlInterface.
func (r *realSlurmControl) IsNodeDrain(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error) {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do IsNodeDrain()",
			"pod", klog.KObj(pod))
		return false, nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		return false, err
	}

	isDrain := slurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateDRAIN)
	return isDrain, nil
}

// IsNodeDrained implements SlurmControlInterface.
func (r *realSlurmControl) IsNodeDrained(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error) {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do IsNodeDrained()",
			"pod", klog.KObj(pod))
		return false, nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		return false, err
	}

	// Drained is when a node has the DRAIN flag and is not doing any work (e.g. job step, prolog, epilog).
	// https://github.com/SchedMD/slurm/blob/slurm-25.05/src/common/slurm_protocol_defs.c#L3500
	isBusy := slurmNode.GetStateAsSet().HasAny(slurmapi.V0044NodeStateALLOCATED, slurmapi.V0044NodeStateMIXED, slurmapi.V0044NodeStateCOMPLETING)
	isDrain := slurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateDRAIN) && !slurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateUNDRAIN)
	isDrained := isDrain && !isBusy

	return isDrained, nil
}

// IsNodeDownForUnresponsive implements SlurmControlInterface.
func (r *realSlurmControl) IsNodeDownForUnresponsive(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error) {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do IsNodeDrained()",
			"pod", klog.KObj(pod))
		return true, nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return true, nil
		}
		return false, err
	}

	// Slurm sets unresponsive nodes as `State=DOWN`, `Reason+="Not responding"`.
	// https://github.com/SchedMD/slurm/blob/slurm-25.05/src/slurmctld/ping_nodes.c#L243
	isDown := slurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateDOWN)
	reasonNotResponding := strings.Contains(ptr.Deref(slurmNode.Reason, ""), "Not responding")
	wasUnresponsive := isDown && reasonNotResponding

	return wasUnresponsive, nil
}

// IsNodeReasonOurs implements SlurmControlInterface.
func (r *realSlurmControl) IsNodeReasonOurs(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error) {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do IsNodeReasonOurs()",
			"pod", klog.KObj(pod))
		return true, nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return true, nil
		}
		return false, err
	}

	// The operator will always prefix the node reason.
	// External sources may not have a prefix or a different one.
	nodeReason := ptr.Deref(slurmNode.Reason, "")
	if nodeReason != "" && !strings.HasPrefix(nodeReason, nodeReasonPrefix) {
		return false, nil
	}

	return true, nil
}

type SlurmNodeStatus struct {
	Total int32

	// Base State
	Allocated int32
	Down      int32
	Error     int32
	Future    int32
	Idle      int32
	Mixed     int32
	Unknown   int32

	// Flag State
	Completing    int32
	Drain         int32
	Fail          int32
	Invalid       int32
	InvalidReg    int32
	Maintenance   int32
	NotResponding int32
	Undrain       int32

	// Per-node State as Conditions
	NodeStates map[string][]corev1.PodCondition
}

// CalculateNodeStatus implements SlurmControlInterface.
func (r *realSlurmControl) CalculateNodeStatus(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) (SlurmNodeStatus, error) {
	logger := log.FromContext(ctx)
	status := SlurmNodeStatus{
		NodeStates: make(map[string][]corev1.PodCondition),
	}

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do CalculateNodeStatus()")
		return status, nil
	}

	nodeList := &slurmtypes.V0044NodeList{}
	if err := slurmClient.List(ctx, nodeList); err != nil {
		if tolerateError(err) {
			return status, nil
		}
		return status, err
	}

	podNodeNameSet := set.New[string]()
	for _, pod := range pods {
		podNodeName := nodesetutils.GetNodeName(pod)
		podNodeNameSet.Insert(podNodeName)
	}

	for _, node := range nodeList.Items {
		nodeName := ptr.Deref(node.Name, "")
		if !podNodeNameSet.Has(nodeName) {
			continue
		}
		status.Total++
		// Slurm Node Base States
		switch {
		case node.GetStateAsSet().Has(slurmapi.V0044NodeStateALLOCATED):
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionAllocated))
			status.Allocated++
		case node.GetStateAsSet().Has(slurmapi.V0044NodeStateDOWN):
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionDown))
			status.Down++
		case node.GetStateAsSet().Has(slurmapi.V0044NodeStateERROR):
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionError))
			status.Error++
		case node.GetStateAsSet().Has(slurmapi.V0044NodeStateFUTURE):
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionFuture))
			status.Future++
		case node.GetStateAsSet().Has(slurmapi.V0044NodeStateIDLE):
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionIdle))
			status.Idle++
		case node.GetStateAsSet().Has(slurmapi.V0044NodeStateMIXED):
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionMixed))
			status.Mixed++
		case node.GetStateAsSet().Has(slurmapi.V0044NodeStateUNKNOWN):
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionUnknown))
			status.Unknown++
		}
		// Slurm Node Flag State
		if node.GetStateAsSet().Has(slurmapi.V0044NodeStateCOMPLETING) {
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionCompleting))
			status.Completing++
		}
		if node.GetStateAsSet().Has(slurmapi.V0044NodeStateDRAIN) {
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionDrain))
			status.Drain++
		}
		if node.GetStateAsSet().Has(slurmapi.V0044NodeStateFAIL) {
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionFail))
			status.Fail++
		}
		if node.GetStateAsSet().Has(slurmapi.V0044NodeStateINVALID) {
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionInvalid))
			status.Invalid++
		}
		if node.GetStateAsSet().Has(slurmapi.V0044NodeStateINVALIDREG) {
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionInvalidReg))
			status.InvalidReg++
		}
		if node.GetStateAsSet().Has(slurmapi.V0044NodeStateMAINTENANCE) {
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionMaintenance))
			status.Maintenance++
		}
		if node.GetStateAsSet().Has(slurmapi.V0044NodeStateNOTRESPONDING) {
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionNotResponding))
			status.NotResponding++
		}
		if node.GetStateAsSet().Has(slurmapi.V0044NodeStateUNDRAIN) {
			status.NodeStates[nodeName] = append(status.NodeStates[nodeName],
				nodeState(node, slurmconditions.PodConditionUndrain))
			status.Undrain++
		}
	}

	return status, nil
}

const infiniteDuration = time.Duration(math.MaxInt64)

// GetNodeDeadlines implements SlurmControlInterface.
func (r *realSlurmControl) GetNodeDeadlines(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) (*timestore.TimeStore, error) {
	logger := log.FromContext(ctx)
	ts := timestore.NewTimeStore(timestore.Greater)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do GetNodeDeadlines()")
		return ts, nil
	}

	slurmNodeNamesSet := set.New[string]()
	for _, pod := range pods {
		slurmNodeName := nodesetutils.GetNodeName(pod)
		slurmNodeNamesSet.Insert(slurmNodeName)
	}

	jobList := &slurmtypes.V0044JobInfoList{}
	if err := slurmClient.List(ctx, jobList); err != nil {
		return nil, err
	}

	for _, job := range jobList.Items {
		if !job.GetStateAsSet().Has(slurmapi.V0044JobInfoJobStateRUNNING) {
			continue
		}
		slurmNodeNames, err := hostlist.Expand(ptr.Deref(job.Nodes, ""))
		if err != nil {
			logger.Error(err, "failed to expand job node hostlist",
				"job", ptr.Deref(job.JobId, 0))
			return nil, err
		}
		if !slurmNodeNamesSet.HasAny(slurmNodeNames...) {
			continue
		}

		// Get startTime, when the job was launched on the Slurm worker.
		startTime_NoVal := ptr.Deref(job.StartTime, slurmapi.V0044Uint64NoValStruct{})
		startTime := time.Unix(ptr.Deref(startTime_NoVal.Number, 0), 0)
		// Get the timeLimit, the wall time of the job.
		timeLimit_NoVal := ptr.Deref(job.TimeLimit, slurmapi.V0044Uint32NoValStruct{})
		timeLimit := time.Duration(ptr.Deref(timeLimit_NoVal.Number, 0)) * time.Minute
		if ptr.Deref(timeLimit_NoVal.Infinite, false) {
			timeLimit = infiniteDuration
		}

		// Push time/duration into the fancy map for each node allocated to the job.
		for _, slurmNodeName := range slurmNodeNames {
			ts.Push(slurmNodeName, startTime.Add(timeLimit))
		}
	}

	return ts, nil
}

func (r *realSlurmControl) lookupClient(nodeset *slinkyv1beta1.NodeSet) slurmclient.Client {
	return r.clientMap.Get(nodeset.Spec.ControllerRef.NamespacedName())
}

var _ SlurmControlInterface = &realSlurmControl{}

func NewSlurmControl(clusters *clientmap.ClientMap) SlurmControlInterface {
	return &realSlurmControl{
		clientMap: clusters,
	}
}

func tolerateError(err error) bool {
	if err == nil {
		return true
	}
	errText := err.Error()
	if errText == http.StatusText(http.StatusNotFound) ||
		errText == http.StatusText(http.StatusNoContent) {
		return true
	}
	return false
}

// Translate a Slurm node state to a plaintext state with a reason
// and a flag to indicate if it is a base state or a flag state.
func nodeState(node slurmtypes.V0044Node, condType corev1.PodConditionType) corev1.PodCondition {
	return corev1.PodCondition{
		Type:    condType,
		Status:  corev1.ConditionTrue,
		Message: ptr.Deref(node.Reason, ""),
	}
}
