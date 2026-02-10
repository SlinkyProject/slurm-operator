// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	NodeSetKind = "NodeSet"
)

var (
	NodeSetGVK        = GroupVersion.WithKind(NodeSetKind)
	NodeSetAPIVersion = GroupVersion.String()
)

// NodeSetSpec defines the desired state of NodeSet
type NodeSetSpec struct {
	// controllerRef is a reference to the Controller CR to which this has membership.
	// +required
	ControllerRef ObjectReference `json:"controllerRef"`

	// replicas is the desired number of replicas of the given Template.
	// These are replicas in the sense that they are instantiations of the
	// same Template, but individual replicas also have a consistent identity.
	// If unspecified, defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// The slurmd container configuration.
	// See corev1.Container spec.
	// Ref: https://github.com/kubernetes/api/blob/master/core/v1/types.go#L2885
	// +optional
	Slurmd ContainerWrapper `json:"slurmd,omitempty"`

	// SSH configuration for worker pods.
	// +optional
	Ssh NodeSetSsh `json:"ssh,omitzero"`

	// The logfile sidecar configuration.
	// +optional
	LogFile ContainerWrapper `json:"logfile,omitzero"`

	// Template is the object that describes the pod that will be created if
	// insufficient replicas are detected.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#pod-template
	// +optional
	Template PodTemplate `json:"template,omitempty"`

	// ExtraConf is added to the slurmd args as `--conf <extraConf>`.
	// Ref: https://slurm.schedmd.com/slurmd.html#OPT_conf-%3Cnode-parameters%3E
	// +optional
	ExtraConf string `json:"extraConf,omitzero"`

	// Partition defines the Slurm partition configuration for this NodeSet.
	// +optional
	Partition NodeSetPartition `json:"partition,omitzero"`

	// volumeClaimTemplates is a list of claims that pods are allowed to reference.
	// The NodeSet controller is responsible for mapping network identities to
	// claims in a way that maintains the identity of a pod. Every claim in
	// this list must have at least one matching (by name) volumeMount in one
	// container in the template. A claim in this list takes precedence over
	// any volumes in the template, with the same name.
	// +nullable
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`

	// updateStrategy indicates the NodeSetUpdateStrategy that will be
	// employed to update Pods in the NodeSet when a revision is made to
	// Template.
	UpdateStrategy NodeSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// revisionHistoryLimit is the maximum number of revisions that will
	// be maintained in the NodeSet's revision history. The revision history
	// consists of all revisions not represented by a currently applied
	// NodeSetSpec version. The default value is 0.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`

	// PersistentVolumeClaimRetentionPolicy describes the policy used for PVCs
	// created from the NodeSet VolumeClaimTemplates. This requires the
	// NodeSetAutoDeletePVC feature gate to be enabled, which is alpha.
	// +optional
	PersistentVolumeClaimRetentionPolicy *NodeSetPersistentVolumeClaimRetentionPolicy `json:"persistentVolumeClaimRetentionPolicy,omitempty"`

	// minReadySeconds is the minimum number of seconds for which a newly
	// created NodeSet Pod should be ready without any of its container crashing,
	// for it to be considered available.
	// Defaults to 0 (pod will be considered available as soon as it is ready).
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`

	// OrdinalPadding indicates how many places to pad with zeroes when constructing the ordinal.
	// +optional
	// +default:=0
	OrdinalPadding uint `json:"ordinalPadding,omitempty"`

	// TaintKubeNodes controls whether or not to apply a NoExecute taint to any nodes which are running a pod from this NodeSet.
	// See https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ for more information.
	// +optional
	// +default:=false
	TaintKubeNodes bool `json:"taintKubeNodes,omitempty"`

	// WorkloadDisruptionProtection controls whether or not pods in this nodeset which are actively running Slurm jobs are protected by
	// a Pod Disruption Budget.
	// See https://kubernetes.io/docs/tasks/run-application/configure-pdb/ for more information.
	// +optional
	// +default:=true
	WorkloadDisruptionProtection bool `json:"workloadDisruptionProtection,omitempty"`
}

// NodeSetPartition defines the Slurm partition configuration for the NodeSet.
type NodeSetPartition struct {
	// Enabled will create a partition for this NodeSet.
	// +default:=true
	Enabled bool `json:"enabled"`

	// Config is added to the NodeSet's partition line.
	// Ref: https://slurm.schedmd.com/slurm.conf.html#SECTION_PARTITION-CONFIGURATION
	// +optional
	Config string `json:"config,omitzero"`
}

// NodeSetSsh defines SSH configuration for NodeSet worker pods.
type NodeSetSsh struct {
	// Enabled controls whether SSH access is enabled for this NodeSet.
	// +default:=false
	Enabled bool `json:"enabled"`

	// ExtraSshdConfig is added to the end of `sshd_config`.
	// Ref: https://man7.org/linux/man-pages/man5/sshd_config.5.html
	// +optional
	ExtraSshdConfig string `json:"extraSshdConfig,omitzero"`

	// SssdConfRef is a reference to a secret containing the `sssd.conf`.
	// +required
	SssdConfRef corev1.SecretKeySelector `json:"sssdConfRef,omitzero"`
}

// NodeSetUpdateStrategy indicates the strategy that the NodeSet
// controller will be used to perform updates. It includes any additional
// parameters necessary to perform the update for the indicated strategy.
type NodeSetUpdateStrategy struct {
	// Type indicates the type of the NodeSetUpdateStrategy.
	// Default is RollingUpdate.
	// +optional
	Type NodeSetUpdateStrategyType `json:"type,omitempty"`

	// RollingUpdate is used to communicate parameters when Type is
	// RollingUpdateNodeSetStrategyType.
	// +optional
	RollingUpdate *RollingUpdateNodeSetStrategy `json:"rollingUpdate,omitempty"`
}

// PersistentVolumeClaimRetentionPolicyType is a string enumeration of the policies that will determine
// when volumes from the VolumeClaimTemplates will be deleted when the controlling NodeSet is
// deleted or scaled down.
type PersistentVolumeClaimRetentionPolicyType string

const (
	// RetainPersistentVolumeClaimRetentionPolicyType is the default
	// PersistentVolumeClaimRetentionPolicy and specifies that
	// PersistentVolumeClaims associated with NodeSet VolumeClaimTemplates
	// will not be deleted.
	RetainPersistentVolumeClaimRetentionPolicyType PersistentVolumeClaimRetentionPolicyType = "Retain"

	// DeletePersistentVolumeClaimRetentionPolicyType specifies that
	// PersistentVolumeClaims associated with NodeSet VolumeClaimTemplates
	// will be deleted in the scenario specified in
	// NodeSetPersistentVolumeClaimPolicy.
	DeletePersistentVolumeClaimRetentionPolicyType PersistentVolumeClaimRetentionPolicyType = "Delete"
)

// NodeSetPersistentVolumeClaimRetentionPolicy describes the policy used for PVCs
// created from the NodeSet VolumeClaimTemplates.
type NodeSetPersistentVolumeClaimRetentionPolicy struct {
	// WhenDeleted specifies what happens to PVCs created from NodeSet
	// VolumeClaimTemplates when the NodeSet is deleted. The default policy
	// of `Retain` causes PVCs to not be affected by NodeSet deletion. The
	// `Delete` policy causes those PVCs to be deleted.
	WhenDeleted PersistentVolumeClaimRetentionPolicyType `json:"whenDeleted,omitempty"`

	// WhenScaled specifies what happens to PVCs created from NodeSet
	// VolumeClaimTemplates when the NodeSet is scaled down. The default
	// policy of `Retain` causes PVCs to not be affected by a scaledown. The
	// `Delete` policy causes the associated PVCs for any excess pods to be
	// deleted.
	WhenScaled PersistentVolumeClaimRetentionPolicyType `json:"whenScaled,omitempty"`
}

// NodeSetUpdateStrategyType is a string enumeration type that enumerates
// all possible update strategies for the NodeSet controller.
// +enum
type NodeSetUpdateStrategyType string

const (
	// RollingUpdateNodeSetStrategyType indicates that NodeSet pods will replace
	// the old pods by new ones using a rolling update method
	// (i.e replace pods on each node one after the other).
	RollingUpdateNodeSetStrategyType NodeSetUpdateStrategyType = "RollingUpdate"

	// OnDeleteNodeSetStrategyType indicates that NodeSet pods will only be
	// replaced when the old pod is killed for any reason.
	OnDeleteNodeSetStrategyType NodeSetUpdateStrategyType = "OnDelete"
)

// RollingUpdateNodeSetStrategy is used to communicate parameters for
// RollingUpdateNodeSetStrategyType.
type RollingUpdateNodeSetStrategy struct {
	// The maximum number of pods that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of desired pods (ex: 10%).
	// Absolute number is calculated from percentage by rounding up. This can not be 0.
	// Defaults to 1.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// NodeSetStatus defines the observed state of NodeSet
type NodeSetStatus struct {
	// Total number of non-terminated pods targeted by this NodeSet (their labels match the Selector).
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Total number of non-terminated pods targeted by this NodeSet that have the desired template spec.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// readyReplicas is the number of pods targeted by this NodeSet with a Ready Condition.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Total number of available pods (ready for at least minReadySeconds) targeted by this NodeSet.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// Total number of unavailable pods targeted by this NodeSet. This is the total number of
	// pods that are still required for the NodeSet to have 100% available capacity. They may
	// either be pods that are running but not yet available or pods that still have not been created.
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`

	// The number of NodeSet pods that are running and are in the Slurm IDLE
	// state. IDLE means the Slurm nodes is not ALLOCATED or MIXED, hence is not
	// allocated any Slurm jobs, nor doing work.
	// +optional
	SlurmIdle int32 `json:"slurmIdle,omitempty"`

	// The number of NodeSet pods that are running and are in the Slurm
	// ALLOCATED or MIXED state. ALLOCATED/MIXED means the Slurm node is
	// allocated one or more Slurm jobs and is doing work.
	// +optional
	SlurmAllocated int32 `json:"slurmAllocated,omitempty"`

	// The number of NodeSet pods that are running and are in the Slurm
	// DOWN state. DOWN means the Slurm node is unavailable for use.
	// +optional
	SlurmDown int32 `json:"slurmDown,omitempty"`

	// The number of NodeSet pods that are running and are in the Slurm DRAIN
	// state. DRAIN means the Slurm node becomes unschedulable but allocated
	// Slurm jobs will not be evicted and can continue running until completion.
	// +optional
	SlurmDrain int32 `json:"slurmDrain,omitempty"`

	// observedGeneration is the most recent generation observed for this NodeSet. It corresponds to the
	// NodeSet's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// NodeSetHash is the "controller-revision-hash", which represents the
	// latest version of the NodeSet.
	NodeSetHash string `json:"nodeSetHash"`

	// Count of hash collisions for the NodeSet. The NodeSet controller
	// uses this field as a collision avoidance mechanism when it needs to
	// create the name for the newest ControllerRevision.
	// +optional
	CollisionCount *int32 `json:"collisionCount,omitempty"`

	// Represents the latest available observations of a NodeSet's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Add Selector to status for HPA support in the scale subresource.
	Selector string `json:"selector"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nodesets;nss;slurmd
// +kubebuilder:subresource:scale:specpath=".spec.replicas",statuspath=".status.replicas",selectorpath=".status.selector"
// +kubebuilder:printcolumn:name="REPLICAS",type="integer",JSONPath=".status.replicas",priority=0,description="The current number of pods."
// +kubebuilder:printcolumn:name="UPDATED",type="integer",JSONPath=".status.updatedReplicas",priority=0,description="The number of pods updated."
// +kubebuilder:printcolumn:name="READY",type="integer",JSONPath=".status.readyReplicas",priority=0,description="The number of pods ready."
// +kubebuilder:printcolumn:name="IDLE",type="integer",JSONPath=".status.slurmIdle",priority=1,description="The number of IDLE slurm nodes."
// +kubebuilder:printcolumn:name="ALLOCATED",type="integer",JSONPath=".status.slurmAllocated",priority=1,description="The number of ALLOCATED/MIXED slurm nodes."
// +kubebuilder:printcolumn:name="DOWN",type="integer",JSONPath=".status.slurmDown",priority=1,description="The number of DOWN slurm nodes."
// +kubebuilder:printcolumn:name="DRAIN",type="integer",JSONPath=".status.slurmDrain",priority=1,description="The number of DRAIN slurm nodes."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// NodeSet is the Schema for the nodesets API
type NodeSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeSetSpec   `json:"spec,omitempty"`
	Status NodeSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NodeSetList contains a list of NodeSet
type NodeSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeSet{}, &NodeSetList{})
}
