// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// Prefixes
const (
	SlinkyPrefix = "slinky.slurm.net/"

	NodeSetPrefix  = "nodeset." + SlinkyPrefix
	LoginSetPrefix = "loginset." + SlinkyPrefix
	TopologyPrefix = "topology." + SlinkyPrefix
	FeaturesPrefix = "features." + SlinkyPrefix
)

// Well Known Annotations
const (
	// AnnotationPodCordon indicates NodeSet Pods that should be DRAIN[ING|ED] in Slurm.
	AnnotationPodCordon = NodeSetPrefix + "pod-cordon"

	// LabelPodDeletionCost can be used to set to an int32 that represent the cost of deleting a pod compared to other
	// pods belonging to the same ReplicaSet. Pods with lower deletion cost are preferred to be deleted before pods
	// with higher deletion cost.
	// NOTE: this is honored on a best-effort basis, and does not offer guarantees on pod deletion order.
	// The implicit deletion cost for pods that don't set the annotation is 0, negative values are permitted.
	AnnotationPodDeletionCost = NodeSetPrefix + "pod-deletion-cost"

	// AnnotationPodDeadline stores a time.RFC3339 timestamp, indicating when the Slurm node should complete its running
	// workload by. Pods with an earlier deadline are preferred to be deleted before pods with a later deadline.
	// NOTE: this is honored on a best-effort basis, and does not offer guarantees on pod deletion order.
	AnnotationPodDeadline = NodeSetPrefix + "pod-deadline"
)

// Well Known Annotations for Objects of type corev1.Node
const (
	// AnnotationNodeCordonReason indicates a custom reason for the Slurm DRAIN action taken when the Kube node on which
	// a NodeSet pod is scheduled is cordoned
	AnnotationNodeCordonReason = NodeSetPrefix + "node-cordon-reason"

	// AnnotationNodeTopologySpec indicates the Slurm dynamic topology line (e.g. "topo-switch:s2,topo-block:b2").
	// Ref: https://slurm.schedmd.com/topology.html#dynamic_topo
	AnnotationNodeTopologySpec = TopologyPrefix + "spec"

	// AnnotationNodeFeaturesSpec indicates a comma-separated list of Slurm node features (e.g.
	// "nn-75bfcf47ca3e4f7dc,GPU") for the Slurm node that the NodeSet pod runs on. The operator applies
	// each value as a NodeFeaturePrefix-namespaced feature (e.g. "k8s/nn-75bfcf47ca3e4f7dc") on the Slurm
	// node's available and active features via the REST API, preserving all non-prefixed features.
	// Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_Features
	AnnotationNodeFeaturesSpec = FeaturesPrefix + "spec"

	// AnnotationNodeHostnameOverride may be set to override the pod hostname assigned to NodeSet DaemonSet-mode
	// pod scheduled on the node. When present, the value is used verbatim as the pod's spec.hostname
	// (and therefore the Slurm node name) instead of the default derived from the node name.
	AnnotationNodeHostnameOverride = NodeSetPrefix + "hostname-override"
)

// Well Known Slurm node feature prefixes
const (
	// NodeFeaturePrefix namespaces the Slurm node features the operator manages from
	// AnnotationNodeFeaturesSpec. The operator owns features carrying this prefix
	// (adding, replacing, and removing them to match the annotation) and preserves
	// all others, including the NodeSet baseline, ExtraConf features, and
	// externally-managed features such as NodeFeaturesPlugins. This mirrors the
	// reserved-prefix pattern used for the node Reason ("slurm-operator:").
	NodeFeaturePrefix = "k8s/"
)

// Well Known Labels
const (
	// LabelNodeSetPodName indicates the pod name.
	// NOTE: Set by the NodeSet controller.
	LabelNodeSetPodName = NodeSetPrefix + "pod-name"

	// LabelNodeSetPodIndex indicates the pod's ordinal.
	// NOTE: Set by the NodeSet controller.
	LabelNodeSetPodIndex = NodeSetPrefix + "pod-index"

	// LabelNodeSetPodHostname indicates the pod hostname (used as Slurm node name).
	// NOTE: Set by the NodeSet controller.
	LabelNodeSetPodHostname = NodeSetPrefix + "pod-hostname"

	// LabelNodeSetPodProtect indicates whether the pod is protected against eviction using a PodDisruptionBudget
	// NOTE: Set by the NodeSet controller
	LabelNodeSetPodProtect = NodeSetPrefix + "pod-protect"

	// LabelNodeSetScalingMode indicates the scaling mode (DaemonSet or StatefulSet).
	// NOTE: Set by the NodeSet controller.
	LabelNodeSetScalingMode = NodeSetPrefix + "scaling-mode"
)

// Well Known Finalizers

const (
	// FinalizerNodeSetReservation
	// NOTE: Set by the NodeSet controller.
	FinalizerNodeSetReservation        = NodeSetPrefix + "reservation"
	FinalizerNodeSetJobStateProtection = NodeSetPrefix + "job-state-protection"
)
