// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Prefixes
const (
	SlinkyPrefix = "slinky.slurm.net/"

	NodeSetPrefix  = "nodeset." + SlinkyPrefix
	LoginSetPrefix = "loginset." + SlinkyPrefix
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
)
