// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	SlurmQosKind = "SlurmQos"
)

var (
	SlurmQosGVK        = GroupVersion.WithKind(SlurmQosKind)
	SlurmQosAPIVersion = GroupVersion.String()
)

// SlurmQosSpec defines the desired state of SlurmQos
type SlurmQosSpec struct {
	// controllerRef is a reference to the Controller CR to which this has membership.
	// The referenced Controller must have accountingRef set (QOS requires accounting).
	// +required
	ControllerRef ObjectReference `json:"controllerRef"`

	// description is a human-readable description of the QOS.
	// +optional
	Description string `json:"description,omitempty"`

	// config is the raw QOS configuration passed to the Slurm REST API.
	// Keys map to V0044Qos JSON fields: priority, flags, limits, preempt,
	// usage_factor, usage_threshold, etc.
	// Ref: https://slurm.schedmd.com/rest_api.html
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Config *runtime.RawExtension `json:"config,omitempty"`
}

// SlurmQosStatus defines the observed state of SlurmQos
type SlurmQosStatus struct {
	// ready indicates whether the QOS exists and is synced in Slurm.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// id is the Slurm-assigned QOS ID.
	// +optional
	ID *int32 `json:"id,omitempty"`

	// Represents the latest available observations of a SlurmQos's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=slurmqos
// +kubebuilder:printcolumn:name="READY",type="boolean",JSONPath=".status.ready",priority=0,description="Whether the QOS is synced in Slurm."
// +kubebuilder:printcolumn:name="DESCRIPTION",type="string",JSONPath=".spec.description",priority=1,description="QOS description."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// SlurmQos is the Schema for the slurmqos API
type SlurmQos struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SlurmQosSpec   `json:"spec,omitempty"`
	Status SlurmQosStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SlurmQosList contains a list of SlurmQos
type SlurmQosList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SlurmQos `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SlurmQos{}, &SlurmQosList{})
}
