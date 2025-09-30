// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	LoginSetKind = "LoginSet"
)

var (
	LoginSetGVK        = GroupVersion.WithKind(LoginSetKind)
	LoginSetAPIVersion = GroupVersion.String()
)

// LoginSetSpec defines the desired state of LoginSet
type LoginSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// controllerRef is a reference to the Controller CR to which this has membership.
	// +required
	ControllerRef ObjectReference `json:"controllerRef"`

	// replicas is the desired number of replicas of the given Template.
	// These are replicas in the sense that they are instantiations of the
	// same Template, but individual replicas also have a consistent identity.
	// If unspecified, defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// The login container configuration.
	// See corev1.Container spec.
	// Ref: https://github.com/kubernetes/api/blob/master/core/v1/types.go#L2885
	// +optional
	Login ContainerWrapper `json:"login,omitempty"`

	// Template is the object that describes the pod that will be created if
	// insufficient replicas are detected.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#pod-template
	// +optional
	Template PodTemplate `json:"template,omitempty"`

	// RootSshAuthorizedKeys is `root/.ssh/authorized_keys`.
	// +optional
	RootSshAuthorizedKeys string `json:"rootSshAuthorizedKeys,omitzero"`

	// ExtraSshdConfig is added to the end of `sshd_config`.
	// Ref: https://man7.org/linux/man-pages/man5/sshd_config.5.html
	// +optional
	ExtraSshdConfig string `json:"extraSshdConfig,omitzero"`

	// SssdConfRef is a reference to a secret containing the `sssd.conf`.
	// +required
	SssdConfRef corev1.SecretKeySelector `json:"sssdConfRef,omitzero"`

	// Service defines a template for a Kubernetes Service object.
	// +optional
	Service ServiceSpec `json:"service,omitzero"`
}

// LoginSetStatus defines the observed state of LoginSet
type LoginSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Total number of non-terminated pods targeted by this LoginSet (their labels match the Selector).
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Represents the latest available observations of a LoginSet's current state.
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
// +kubebuilder:deprecatedversion:warning="v1alpha1 is deprecated, use v1beta1."
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=loginsets;lss;sackd
// +kubebuilder:subresource:scale:specpath=".spec.replicas",statuspath=".status.replicas",selectorpath=".status.selector"
// +kubebuilder:printcolumn:name="REPLICAS",type="integer",JSONPath=".status.replicas",priority=0,description="The current number of pods."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// LoginSet is the Schema for the loginsets API
type LoginSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoginSetSpec   `json:"spec,omitempty"`
	Status LoginSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoginSetList contains a list of LoginSet
type LoginSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoginSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LoginSet{}, &LoginSetList{})
}
