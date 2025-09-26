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
	// workload by. Pods an earlier daedline are preferred to be deleted before pods with a later deadline.
	// NOTE: this is honored on a best-effort basis, and does not offer guarantees on pod deletion order.
	AnnotationPodDeadline = NodeSetPrefix + "pod-deadline"

	// AnnotationSlurmNodeDrainState indicates the current drain state of the underlying Slurm node.
	// This annotation is designed to be used for external integration with kubernetes break-fix and
	// maintenance automation tools. External tools can query this annotation to determine
	// when it's safe to perform node maintenance operations.
	// Values: "draining" (jobs are finishing), "drained" (ready for maintenance)
	AnnotationSlurmNodeDrainState = NodeSetPrefix + "slurmnode-drain-state"

	// AnnotationSlurmNodeDrainReason indicates why the underlying Slurm node was drained.
	// This annotation provides context for external automation tools about the trigger
	// that initiated the drain operation (e.g., K8s node cordoning).
	AnnotationSlurmNodeDrainReason = NodeSetPrefix + "slurmnode-drain-reason"
)

// Well Known Labels
const (
	// LabelNodeSetPodName indicates the pod name.
	// NOTE: Set by the NodeSet controller.
	LabelNodeSetPodName = NodeSetPrefix + "pod-name"

	// LabelNodeSetPodIndex indicates the pod's ordinal.
	// NOTE: Set by the NodeSet controller.
	LabelNodeSetPodIndex = NodeSetPrefix + "pod-index"
)

// AnnotationSlurmNodeDrainState value type
type AnnotationSlurmNodeDrainStateValue string

// AnnotationSlurmNodeDrainState value enum
const (
	// AnnotationSlurmNodeDrainStateDraining indicates the Slurm node is currently draining.
	// Jobs are finishing and the node is not accepting new work. External tools should
	// wait for the "drained" state before performing maintenance operations.
	AnnotationSlurmNodeDrainStateDraining AnnotationSlurmNodeDrainStateValue = "draining"

	// AnnotationSlurmNodeDrainStateDrained indicates the Slurm node has been fully drained.
	// All jobs have completed and the node is ready for maintenance operations.
	// External break-fix tools can safely proceed with node maintenance at this point.
	AnnotationSlurmNodeDrainStateDrained AnnotationSlurmNodeDrainStateValue = "drained"
)

// AnnotationSlurmNodeDrainReason value type
type AnnotationSlurmNodeDrainReasonValue string

// AnnotationSlurmNodeDrainReason value enum
const (
	// AnnotationSlurmNodeDrainReasonK8sCordon indicates the drain was triggered by
	// Kubernetes node cordoning (e.g., by cloud vendor break-fix automation).
	AnnotationSlurmNodeDrainReasonK8sCordon AnnotationSlurmNodeDrainReasonValue = "k8s-node-cordoned"
)
