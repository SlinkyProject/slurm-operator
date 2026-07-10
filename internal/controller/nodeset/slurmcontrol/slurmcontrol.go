// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmcontrol

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/puttsk/hostlist"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	slurmobject "github.com/SlinkyProject/slurm-client/pkg/object"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podinfo"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/timestore"
	slurmconditions "github.com/SlinkyProject/slurm-operator/pkg/conditions"
)

type SlurmControlInterface interface {
	// RefreshNodeCache forces the Node cache to be refreshed
	RefreshNodeCache(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) error
	// UpdateNodeWithPodInfo handles updating the Node with its pod info
	UpdateNodeWithPodInfo(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error
	// UpdateNodeTopology handles updating the Node with its topologySpec.
	UpdateNodeTopology(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, topologySpec string) error
	// UpdateNodeFeatures reconciles the prefix-namespaced Slurm node features to the
	// given (unprefixed) values, preserving all features outside the prefix.
	UpdateNodeFeatures(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, prefix string, features []string) error
	// MakeNodeDrain handles adding the DRAIN state to the slurm node.
	MakeNodeDrain(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, reason string, overrideReason bool) error
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
	// GetNodesForPods returns a list of Slurm nodes associated with the NodeSet pods.
	GetNodesForPods(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) ([]string, bool, error)
	// CheckReservationForNodeSet returns true when a reservation exists for a NodeSet
	CheckReservationForNodeSet(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) (bool, error)
	// GetPodsUnderReservation returns a sublist of pods whose Slurm nodes are under an active MAINT reservation
	GetPodsUnderReservation(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) ([]*corev1.Pod, error)
	// SyncReservationForNodeSet creates a reservation for a NodeSet for the Scheduled update strategy
	SyncReservationForNodeSet(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) error
	// DeleteReservationForNodeSet deletes a reservation associated with a NodeSet for the Scheduled update strategy
	DeleteReservationForNodeSet(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) error
	// GetDefunctNodesForNodeSet returns defunct-node candidates owned by this NodeSet.
	GetDefunctNodesForNodeSet(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) ([]DefunctNode, bool, error)
	// DeleteNode deletes a Slurm node by name.
	DeleteNode(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, nodeName string) error
	// IsNodeSetSafeToDelete reports whether a NodeSet can be safely deleted without leaving Slurm job state references.
	IsNodeSetSafeToDelete(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) (bool, time.Duration, error)
}

// realSlurmControl is the default implementation of SlurmControlInterface.
type realSlurmControl struct {
	clientMap *clientmap.ClientMap
}

const (
	// defaultMinJobAge is the default wait time before a deleting NodeSet is considered safe to remove.
	defaultMinJobAge = 5 * time.Minute

	// nodeSetDeleteRetryAfter is the retry interval when NodeSet deletion is not yet safe.
	nodeSetDeleteRetryAfter = 30 * time.Second
)

// IsNodeSetSafeToDelete reports whether a NodeSet can be safely removed from the Slurm configuration.
func (r *realSlurmControl) IsNodeSetSafeToDelete(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
) (bool, time.Duration, error) {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do IsNodeSetSafeToDelete()")
		return false, nodeSetDeleteRetryAfter, nil
	}

	partitionName := common.GetSlurmNodeSetName(nodeset)

	jobList := &slurmtypes.V0044JobInfoList{}
	if err := slurmClient.List(ctx, jobList); err != nil {
		return false, nodeSetDeleteRetryAfter, err
	}

	for _, job := range jobList.Items {
		if ptr.Deref(job.Partition, "") == partitionName {
			return false, nodeSetDeleteRetryAfter, nil
		}
	}

	if nodeset.DeletionTimestamp == nil {
		return false, nodeSetDeleteRetryAfter, nil
	}

	now := time.Now()
	waitUntil := nodeset.DeletionTimestamp.Time.Add(defaultMinJobAge)
	if now.Before(waitUntil) {
		return false, waitUntil.Sub(now), nil
	}

	return true, 0, nil
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
	key := slurmobject.ObjectKey(nodesetutils.GetSlurmNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	podInfo := podinfo.PodInfo{
		Namespace:   pod.GetNamespace(),
		PodName:     pod.GetName(),
		Node:        pod.Spec.NodeName,
		NodeSetName: nodeset.Name,
		NodeSetUID:  string(nodeset.UID),
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

// UpdateNodeTopology implements SlurmControlInterface.
func (r *realSlurmControl) UpdateNodeTopology(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, topologySpec string) error {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do UpdateNodeTopology()",
			"pod", klog.KObj(pod))
		return nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetSlurmNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	nodeTopology := ptr.Deref(slurmNode.Topology, "")
	if apiequality.Semantic.DeepEqual(nodeTopology, topologySpec) {
		logger.V(3).Info("Node topologySpec is identical to request, skipping update request",
			"node", slurmNode.GetKey(), "topologySpec", nodeTopology)
		return nil
	}

	logger.Info("Update Slurm Node topologySpec", "Node", slurmNode.GetKey(), "topologySpec", topologySpec)
	req := slurmapi.V0044UpdateNodeMsg{
		TopologyStr: ptr.To(topologySpec),
	}
	if err := slurmClient.Update(ctx, slurmNode, req); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	return nil
}

// UpdateNodeFeatures implements SlurmControlInterface.
func (r *realSlurmControl) UpdateNodeFeatures(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, prefix string, features []string) error {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do UpdateNodeFeatures()",
			"pod", klog.KObj(pod))
		return nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetSlurmNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	// The operator owns only the `prefix` namespace. Recompute the available and
	// active feature sets by replacing the prefixed features with the desired set
	// and preserving everything else: the NodeSet baseline (seeded at slurmd
	// registration via --conf), ExtraConf features, and externally-managed features
	// such as those from NodeFeaturesPlugins.
	desired := make([]string, 0, len(features))
	for _, f := range features {
		desired = append(desired, prefix+f)
	}
	curAvailable := ptr.Deref(slurmNode.Features, slurmapi.V0044CsvString{})
	curActive := ptr.Deref(slurmNode.ActiveFeatures, slurmapi.V0044CsvString{})
	newAvailable := replaceFeatureNamespace(curAvailable, prefix, desired)
	newActive := replaceFeatureNamespace(curActive, prefix, desired)

	// Features are a set; compare order-insensitively so we only update when the
	// prefixed namespace actually changed.
	isAvailableInSync := set.New(newAvailable...).Equal(set.New(curAvailable...))
	isActiveInSync := set.New(newActive...).Equal(set.New(curActive...))
	if isAvailableInSync && isActiveInSync {
		logger.V(3).Info("Node features already in sync, skipping update request",
			"node", slurmNode.GetKey(), "features", desired)
		return nil
	}

	logger.Info("Update Slurm Node features", "Node", slurmNode.GetKey(), "features", desired)
	req := slurmapi.V0044UpdateNodeMsg{
		Features:    new(slurmapi.V0044CsvString(newAvailable)),
		FeaturesAct: new(slurmapi.V0044CsvString(newActive)),
	}
	if err := slurmClient.Update(ctx, slurmNode, req); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	return nil
}

// replaceFeatureNamespace returns current with all features carrying prefix replaced
// by desired (already prefixed), preserving every feature outside the prefix. The
// result is sorted and de-duplicated.
func replaceFeatureNamespace(current []string, prefix string, desired []string) []string {
	preserved := make([]string, 0, len(current))
	for _, f := range current {
		if !strings.HasPrefix(f, prefix) {
			preserved = append(preserved, f)
		}
	}
	return structutils.SortedDedup(structutils.MergeList(preserved, desired))
}

const nodeReasonPrefix = "slurm-operator: "

// MakeNodeDrain implements SlurmControlInterface.
func (r *realSlurmControl) MakeNodeDrain(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, reason string, overrideReason bool) error {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do MakeNodeDrain()",
			"pod", klog.KObj(pod))
		return nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetSlurmNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	nodeReason := ptr.Deref(slurmNode.Reason, "")
	newReason := FormatNodeReason(reason)
	if !overrideReason && nodeReason != "" {
		newReason = nodeReason
	}

	// If Slurm node is already drained and the reasons match, no need to drain it again
	if slurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateDRAIN) && nodeReason == newReason {
		logger.V(1).Info("Node is already drained, skipping drain request",
			"node", slurmNode.GetKey(), "nodeState", slurmNode.State, "nodeReason", nodeReason)
		return nil
	}

	logger.V(1).Info("make slurm node drain",
		"pod", klog.KObj(pod))
	req := slurmapi.V0044UpdateNodeMsg{
		State:  ptr.To([]slurmapi.V0044UpdateNodeMsgState{slurmapi.V0044UpdateNodeMsgStateDRAIN}),
		Reason: ptr.To(newReason),
	}
	if err := slurmClient.Update(ctx, slurmNode, req); err != nil {
		if tolerateError(err) {
			return nil
		}
		return err
	}

	return nil
}

func FormatNodeReason(reason string) string {
	return nodeReasonPrefix + reason
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
	key := slurmobject.ObjectKey(nodesetutils.GetSlurmNodeName(pod))
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
		prefixedReason = FormatNodeReason(reason)
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
		return true, nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetSlurmNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return true, nil
		}
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
		return true, nil
	}

	slurmNode := &slurmtypes.V0044Node{}
	key := slurmobject.ObjectKey(nodesetutils.GetSlurmNodeName(pod))
	if err := slurmClient.Get(ctx, key, slurmNode); err != nil {
		if tolerateError(err) {
			return true, nil
		}
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
	key := slurmobject.ObjectKey(nodesetutils.GetSlurmNodeName(pod))
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
	key := slurmobject.ObjectKey(nodesetutils.GetSlurmNodeName(pod))
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
		podNodeName := nodesetutils.GetSlurmNodeName(pod)
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
		slurmNodeName := nodesetutils.GetSlurmNodeName(pod)
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

// GetNodesForPods implements SlurmControlInterface.
func (r *realSlurmControl) GetNodesForPods(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) ([]string, bool, error) {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do GetNodesForPods()")
		return nil, false, nil
	}

	nodeList := &slurmtypes.V0044NodeList{}
	if err := slurmClient.List(ctx, nodeList); err != nil {
		return nil, true, err
	}

	// Expected Slurm nodes backed by NodeSet pods
	podNodeNameSet := set.New[string]()
	for _, pod := range pods {
		podNodeName := nodesetutils.GetSlurmNodeName(pod)
		podNodeNameSet.Insert(podNodeName)
	}

	// Actual Slurm nodes given NodeSet pods
	slurmNodeNames := []string{}
	for _, node := range nodeList.Items {
		nodeName := ptr.Deref(node.Name, "")
		if !podNodeNameSet.Has(nodeName) {
			continue
		}
		slurmNodeNames = append(slurmNodeNames, nodeName)
	}

	return slurmNodeNames, true, nil
}

type DefunctNode struct {
	Name    string
	PodInfo podinfo.PodInfo
}

// GetDefunctNodesForNodeSet implements SlurmControlInterface.
func (r *realSlurmControl) GetDefunctNodesForNodeSet(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) ([]DefunctNode, bool, error) {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do GetDefunctNodesForNodeSet()")
		return nil, false, nil
	}

	nodeList := &slurmtypes.V0044NodeList{}
	if err := slurmClient.List(ctx, nodeList); err != nil {
		return nil, true, err
	}

	defunctNodes := make([]DefunctNode, 0)
	for _, node := range nodeList.Items {
		if !node.GetStateAsSet().HasAll(slurmapi.V0044NodeStateDOWN, slurmapi.V0044NodeStateNOTRESPONDING) {
			continue
		}

		info := &podinfo.PodInfo{}
		if err := podinfo.ParseIntoPodInfo(node.Comment, info); err != nil {
			continue
		}
		if info.Namespace != nodeset.Namespace ||
			info.PodName == "" ||
			info.NodeSetName != nodeset.Name ||
			info.NodeSetUID == "" ||
			info.NodeSetUID != string(nodeset.UID) {
			continue
		}

		nodeName := ptr.Deref(node.Name, "")
		if nodeName == "" {
			continue
		}

		defunctNodes = append(defunctNodes, DefunctNode{
			Name:    nodeName,
			PodInfo: *info,
		})
	}

	return defunctNodes, true, nil
}

// DeleteNode implements SlurmControlInterface.
func (r *realSlurmControl) DeleteNode(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, nodeName string) error {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do DeleteNode()", "nodeName", nodeName)
		return nil
	}

	slurmNode := &slurmtypes.V0044Node{
		V0044Node: slurmapi.V0044Node{
			Name: new(nodeName),
		},
	}
	if err := slurmClient.Delete(ctx, slurmNode); err != nil && !tolerateError(err) {
		return err
	}

	return nil
}

// CheckReservationForNodeSet returns the state of the reservation
// for the given nodeset in Slurm.
func (r *realSlurmControl) CheckReservationForNodeSet(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) (bool, error) {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do CheckReservationForNodeSet()")
		return false, nil
	}

	emptyReservation := new(slurmtypes.V0044ReservationInfo)
	reservation := new(slurmtypes.V0044ReservationInfo)

	key := slurmobject.ObjectKey("SlurmOperatorMaint-" + nodeset.Name)
	if err := slurmClient.Get(ctx, key, reservation); err != nil {
		if tolerateError(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	if !apiequality.Semantic.DeepEqual(reservation, emptyReservation) {
		return true, nil
	}

	return false, nil
}

// GetPodsUnderReservation() returns a sublist of pods corresponding to Slurm nodes actively under
// a MAINT reservation
func (r *realSlurmControl) GetPodsUnderReservation(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) ([]*corev1.Pod, error) {
	logger := log.FromContext(ctx)

	var podsUnderReservation []*corev1.Pod

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do GetPodsUnderReservation()")
		return nil, nil
	}

	reservation := new(slurmtypes.V0044ReservationInfo)
	key := slurmobject.ObjectKey("SlurmOperatorMaint-" + nodeset.Name)
	if err := slurmClient.Get(ctx, key, reservation); !tolerateError(err) {
		return nil, err
	}
	if reservation.Name == nil {
		return nil, nil
	}

	// For each pod, determine if the associated Slurm node is actively under the NodeSet's reservation
	for _, pod := range pods {
		nodename := nodesetutils.GetSlurmNodeName(pod)

		slurmNode := new(slurmtypes.V0044Node)
		key := slurmobject.ObjectKey(nodename)
		if err := slurmClient.Get(ctx, key, slurmNode); !tolerateError(err) {
			return nil, err
		}
		if slurmNode.State != nil && slurmNode.Reservation != nil {
			if slurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateMAINTENANCE) && *slurmNode.Reservation == *reservation.Name {
				podsUnderReservation = append(podsUnderReservation, pod)
			}
		}
	}

	return podsUnderReservation, nil
}

// DeleteReservationForNodeSet() deletes the reservation associated with a NodeSet
func (r *realSlurmControl) DeleteReservationForNodeSet(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) error {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do DeleteReservationForNodeSet()")
		return nil
	}

	reservation := new(slurmtypes.V0044ReservationInfo)
	key := slurmobject.ObjectKey("SlurmOperatorMaint-" + nodeset.Name)
	if err := slurmClient.Get(ctx, key, reservation); !tolerateError(err) {
		return err
	}

	if reservation.Name == nil {
		return nil
	}

	if err := slurmClient.Delete(ctx, reservation); !tolerateError(err) {
		return err
	}

	return nil
}

// SyncReservationForNodeSet() creates and updates the reservation associated with a NodeSet
func (r *realSlurmControl) SyncReservationForNodeSet(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pods []*corev1.Pod) error {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(nodeset)
	if slurmClient == nil {
		logger.V(2).Info("no client for nodeset, cannot do SyncReservationForNodeSet()")
		return nil
	}

	name := "SlurmOperatorMaint-" + nodeset.Name

	reservationDesc, newReservationInfo, err := formatReservationForSchedule(name, nodeset.Spec.UpdateStrategy.ScheduledUpdate)
	if err != nil {
		return fmt.Errorf("SyncReservationForNodeSet() failed to format Reservation=%s for NodeSet=%s with error=%w", *reservationDesc.Name, nodeset.Name, err)
	}

	slurmNodes, ok, err := r.GetNodesForPods(ctx, nodeset, pods)
	if err != nil {
		return err
	} else if !ok {
		return nil // skip, results cannot be used
	}
	slurmNodeHostList, err := hostlist.Compress(slurmNodes)
	if err != nil {
		return err
	}
	reservationDesc.NodeList = &slurmapi.V0044HostlistString{slurmNodeHostList}
	newReservationInfo.NodeList = ptr.To(slurmNodeHostList)

	coreNewReservationInfo := slurmtypes.V0044ReservationInfo{
		V0044ReservationInfo: slurmapi.V0044ReservationInfo{
			Name:     newReservationInfo.Name,
			Flags:    newReservationInfo.Flags,
			NodeList: newReservationInfo.NodeList,
			Users:    newReservationInfo.Users,
		},
	}

	// If a reservation already exists for this NodeSet, get information to determine what actions to take
	var coreOldReservationInfo slurmtypes.V0044ReservationInfo
	var reservationActive bool

	oldReservationInfo := new(slurmtypes.V0044ReservationInfo)
	key := slurmobject.ObjectKey("SlurmOperatorMaint-" + nodeset.Name)
	if err := slurmClient.Get(ctx, key, oldReservationInfo); !tolerateError(err) {
		return err
	}

	// We need to append the output-only flag SPEC_NODES to our newReservationInfo, to match what we
	// expect from a populated oldReservationInfo
	specNodeFlag := []slurmapi.V0044ReservationInfoFlags{slurmapi.V0044ReservationInfoFlagsSPECNODES}
	if coreNewReservationInfo.Flags != nil {
		coreNewReservationInfo.Flags = ptr.To(append(*coreNewReservationInfo.Flags, specNodeFlag...))
		// We need to sort the slice of flags in order for the comparison that gates updates to evaluate properly
		coreNewReservationInfo.Flags = ptr.To(set.New(*coreNewReservationInfo.Flags...).SortedList())
	}

	var startTimeChanged bool
	emptyReservationInfo := new(slurmtypes.V0044ReservationInfo)

	slurmReservationExists := !apiequality.Semantic.DeepEqual(oldReservationInfo, emptyReservationInfo)
	created, startTime := getReservationStatus(nodeset)

	switch {
	case slurmReservationExists && created:
		// coreOldReservationInfo is used to compare key values of the new and old reservations
		// We rely on startTimeChanged instead of comparing the StartTime's directly to ensure
		// that the automatic time change on reservation reoccurance does not cause constant,
		// unnecessary reservation updates
		coreOldReservationInfo = slurmtypes.V0044ReservationInfo{
			V0044ReservationInfo: slurmapi.V0044ReservationInfo{
				Name:     oldReservationInfo.Name,
				Flags:    oldReservationInfo.Flags,
				NodeList: oldReservationInfo.NodeList,
				Users:    oldReservationInfo.Users,
			},
		}

		if coreOldReservationInfo.Flags != nil {
			// We need to sort the slice of flags in order for the comparison that gates updates to evaluate properly
			coreOldReservationInfo.Flags = ptr.To(set.New(*coreOldReservationInfo.Flags...).SortedList())
		}

		reservationActive = isReservationActive(*oldReservationInfo)
		startTimeChanged = !nodeset.Spec.UpdateStrategy.ScheduledUpdate.StartTime.Time.Equal(startTime)

		// We should honor existing start times for reoccuring reservations, Slurm updates them for us.
		// This approach is required until we have a slurmapi.V0044ReservationInfo.Status field from Slurm
		// due to the scenario outlined below:
		//
		// 1. Reservation has not yet occurred. Slurm's reservation record matches the NodeSet object's StartTime.
		// 2. Reservation is ACTIVE. Reservation Updates are skipped during NodeSet reconciliation because
		//    of this.
		// 3. Reservation completes. Because this is a reoccuring reservation, Slurm automatically updates the
		//    reservation's StartTime for the next occurrence of the reservation (in the future).
		// 4. NodeSet reconciliation occurs and this function is called. The apiequality.Semantic.DeepEqual()
		//    call that is used in order to compare the reservation as defined in the NodeSet object and the
		//    reservation as it exists in Slurm will now evaluate to false, triggering a reservation update.
		//    This update will fail because even with the FORCE_START flag set, Slurm will not permit a start time
		//    to be in the past for a reoccuring reservation once the StartTime has been set by Slurm after the
		//    first occurrence.
		oldStartTime := time.Unix(*oldReservationInfo.StartTime.Number, 0)
		if oldStartTime.After(time.Unix(*reservationDesc.StartTime.Number, 0)) {
			oldResIsReoccuring := reservationHasFlags(*oldReservationInfo, reoccuringInfoFlags, true)
			newResIsReoccuring := reservationHasFlags(newReservationInfo, reoccuringInfoFlags, true)
			if (oldResIsReoccuring && newResIsReoccuring) && !startTimeChanged {
				newStartTime := oldStartTime.Unix()
				reservationDesc.StartTime.Number = &newStartTime
			}
		}

		// Slurm will not allow most field updates for active reservations
		if !reservationActive && (!apiequality.Semantic.DeepEqual(coreNewReservationInfo, coreOldReservationInfo) || startTimeChanged) {

			newReservationInfo.Name = oldReservationInfo.Name

			err = slurmClient.Update(ctx, &newReservationInfo, reservationDesc)
			if !tolerateError(err) {
				return fmt.Errorf("SyncReservationForNodeSet() failed to Update ReservationName=%s for NodeSet=%s with error=%w", ptr.Deref(reservationDesc.Name, name), nodeset.Name, err)
			}

			return nil
		}

		// Adding and removing nodes from an active reservation is permitted by Slurm, this should be done to maintain sync
		if reservationActive && !isNodeListMatch(ptr.Deref(oldReservationInfo, slurmtypes.V0044ReservationInfo{}), newReservationInfo) {
			err := updateReservationNodes(ctx, slurmClient, oldReservationInfo, reservationDesc.NodeList)
			if !tolerateError(err) {
				return fmt.Errorf("SyncReservationForNodeSet() failed to Update Reservation=%s for NodeSet=%s with error=%w", *reservationDesc.Name, nodeset.Name, err)
			}
		}

		return nil

	case !slurmReservationExists && !created:
		forceStart := reservationHasFlags(newReservationInfo, []slurmapi.V0044ReservationInfoFlags{slurmapi.V0044ReservationInfoFlagsFORCESTART}, true)
		pastStartTime := nodeset.Spec.UpdateStrategy.ScheduledUpdate.StartTime.Time.Before(time.Now())

		if forceStart || !pastStartTime {
			err = slurmClient.Create(ctx, &newReservationInfo, reservationDesc)
			if !tolerateError(err) {
				return fmt.Errorf("SyncReservationForNodeSet() failed to Create ReservationName=%s for NodeSet=%s with error=%w", ptr.Deref(reservationDesc.Name, name), nodeset.Name, err)
			}
		}

		return nil
	}

	return nil
}

func isNodeListMatch(old slurmtypes.V0044ReservationInfo, new slurmtypes.V0044ReservationInfo) bool {
	if old.NodeList != nil {
		newNodeList, _ := hostlist.Expand(*new.NodeList)
		oldNodeList, _ := hostlist.Expand(*old.NodeList)
		if apiequality.Semantic.DeepEqual(newNodeList, oldNodeList) {
			return true
		}
	}

	return false
}

func updateReservationNodes(ctx context.Context, slurmClient slurmclient.Client, reservation *slurmtypes.V0044ReservationInfo, nodelist *slurmapi.V0044HostlistString) error {

	var flags []slurmapi.V0044ReservationDescMsgFlags
	if reservation.Flags != nil {
		for _, f := range *reservation.Flags {
			flags = append(flags, slurmapi.V0044ReservationDescMsgFlags(f))
		}
	}

	oldReservation := slurmapi.V0044ReservationDescMsg{
		Name:      reservation.Name,
		StartTime: reservation.StartTime,
		EndTime:   reservation.EndTime,
		Flags:     &flags,
		Users:     ptr.To(slurmapi.V0044CsvString{common.SlurmUser}),
		NodeList:  nodelist,
	}

	err := slurmClient.Update(ctx, reservation, oldReservation)
	if err != nil {
		return err
	}

	return nil
}

// isReservationActive() returns a boolean value based on whether the reservation
// that it is passed is currently running. Once the Slurm RestAPI provides a
// slurmapi.V0044ReservationInfoStatus field, this helper function should be
// deleted in favor of using that field directly.
func isReservationActive(reservation slurmtypes.V0044ReservationInfo) bool {
	start := time.Unix(*reservation.StartTime.Number, 0)
	end := time.Unix(*reservation.EndTime.Number, 0)

	now := time.Now().In(time.UTC)
	if start.Before(now) && end.After(now) {
		return true
	}

	return false
}

// getReservationStatus() returns the boolean and time value from the NodeSet's
// ReservationCreated Condition, if set
func getReservationStatus(nodeset *slinkyv1beta1.NodeSet) (bool, time.Time) {
	for i := range nodeset.Status.Conditions {
		if nodeset.Status.Conditions[i].Type == slurmconditions.NodeSetConditionReservationCreated {
			switch nodeset.Status.Conditions[i].Status {
			case metav1.ConditionTrue:
				return true, nodeset.Status.Conditions[i].LastTransitionTime.Time
			case metav1.ConditionFalse:
				return false, nodeset.Status.Conditions[i].LastTransitionTime.Time
			}
		}
	}

	return false, time.Time{}
}

var reoccuringInfoFlags = []slurmapi.V0044ReservationInfoFlags{
	slurmapi.V0044ReservationInfoFlagsHOURLY,
	slurmapi.V0044ReservationInfoFlagsDAILY,
	slurmapi.V0044ReservationInfoFlagsWEEKLY,
	slurmapi.V0044ReservationInfoFlagsWEEKEND,
	slurmapi.V0044ReservationInfoFlagsWEEKDAY,
}

func reservationHasFlags(reservation slurmtypes.V0044ReservationInfo, flags []slurmapi.V0044ReservationInfoFlags, anyFlags bool) bool {
	if reservation.Flags == nil && len(flags) > 0 {
		return false
	}

	flagSet := set.New(flags...)
	resFlags := set.New(*reservation.Flags...)
	intersection := flagSet.Intersection(resFlags)

	if anyFlags {
		return intersection.Len() > 0
	} else {
		return intersection.Len() == len(flags)
	}
}

var (
	resInfoFlags = []slurmapi.V0044ReservationInfoFlags{
		slurmapi.V0044ReservationInfoFlagsMAINT,
		slurmapi.V0044ReservationInfoFlagsIGNOREJOBS,
	}
	resDescFlags = []slurmapi.V0044ReservationDescMsgFlags{
		slurmapi.V0044ReservationDescMsgFlagsMAINT,
		slurmapi.V0044ReservationDescMsgFlagsIGNOREJOBS,
	}
)

func formatReservationForSchedule(name string, schedule slinkyv1beta1.ScheduledUpdateNodeSetStrategy) (slurmapi.V0044ReservationDescMsg, slurmtypes.V0044ReservationInfo, error) {
	var reservationInfo slurmtypes.V0044ReservationInfo
	var reservation slurmapi.V0044ReservationDescMsg

	startTime := timeToUint64NoVal(schedule.StartTime.In(time.UTC))
	endTime := timeToUint64NoVal(schedule.StartTime.In(time.UTC).Add(schedule.Duration.Duration))
	duration := durationToUint32NoVal(schedule.Duration.Duration)

	descFlags := parseReservationFlags(schedule.Flags, resDescFlags)
	infoFlags := parseReservationFlags(schedule.Flags, resInfoFlags)

	reservationInfo = slurmtypes.V0044ReservationInfo{
		V0044ReservationInfo: slurmapi.V0044ReservationInfo{
			Name:      &name,
			StartTime: &startTime,
			EndTime:   &endTime,
			Flags:     &infoFlags,
			Users:     ptr.To(common.SlurmUser),
		},
	}
	reservation = slurmapi.V0044ReservationDescMsg{
		Name:      &name,
		StartTime: &startTime,
		Duration:  &duration,
		Flags:     &descFlags,
		Users:     ptr.To(slurmapi.V0044CsvString{common.SlurmUser}),
	}

	return reservation, reservationInfo, nil
}

func timeToUint64NoVal(t time.Time) slurmapi.V0044Uint64NoValStruct {
	return slurmapi.V0044Uint64NoValStruct{
		Infinite: new(false),
		Number:   ptr.To(t.Unix()),
		Set:      new(true),
	}
}

func durationToUint32NoVal(d time.Duration) slurmapi.V0044Uint32NoValStruct {
	return slurmapi.V0044Uint32NoValStruct{
		Infinite: new(false),
		Number:   ptr.To(int32(d.Minutes())),
		Set:      new(true),
	}
}

func parseReservationFlags[T slurmapi.V0044ReservationInfoFlags | slurmapi.V0044ReservationDescMsgFlags](flags []string, reqFlags []T) []T {
	// Required flags
	flagSet := set.New(reqFlags...)

	// User flags
	for _, flag := range flags {
		f := T(strings.ToUpper(flag))
		flagSet.Insert(f)
	}

	return flagSet.SortedList()
}

func (r *realSlurmControl) lookupClient(nodeset *slinkyv1beta1.NodeSet) slurmclient.Client {
	key := ktypes.NamespacedName{
		Namespace: nodeset.Namespace,
		Name:      nodeset.Spec.ControllerRef.Name,
	}
	return r.clientMap.Get(key)
}

var _ SlurmControlInterface = &realSlurmControl{}

func NewSlurmControl(clientMap *clientmap.ClientMap) SlurmControlInterface {
	return &realSlurmControl{
		clientMap: clientMap,
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
