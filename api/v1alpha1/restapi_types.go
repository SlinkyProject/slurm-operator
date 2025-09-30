// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	RestApiKind = "RestApi"
)

var (
	RestApiGVK        = GroupVersion.WithKind(RestApiKind)
	RestApiAPIVersion = GroupVersion.String()
)

// RestApiSpec defines the desired state of RestApi
type RestApiSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// controllerRef is a reference to the Controller CR to which this has membership.
	// +required
	ControllerRef ObjectReference `json:"controllerRef"`

	// replicas is the desired number of replicas.
	// If unspecified, defaults to 1.
	// +optional
	// +default:=1
	Replicas *int32 `json:"replicas,omitempty"`

	// The slurmrestd container configuration.
	// See corev1.Container spec.
	// Ref: https://github.com/kubernetes/api/blob/master/core/v1/types.go#L2885
	// +optional
	Slurmrestd ContainerWrapper `json:"slurmrestd,omitempty"`

	// Template is the object that describes the pod that will be created if
	// insufficient replicas are detected.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#pod-template
	// +optional
	Template PodTemplate `json:"template,omitempty"`

	// Service defines a template for a Kubernetes Service object.
	// +optional
	Service ServiceSpec `json:"service,omitzero"`
}

// RestApiStatus defines the observed state of Restapi
type RestApiStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Represents the latest available observations of a Restapi's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:deprecatedversion:warning="v1alpha1 is deprecated, use v1beta1."
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=slurmrestd
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Restapi is the Schema for the restapis API
type RestApi struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestApiSpec   `json:"spec,omitempty"`
	Status RestApiStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RestapiList contains a list of Restapi
type RestApiList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RestApi `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RestApi{}, &RestApiList{})
}
