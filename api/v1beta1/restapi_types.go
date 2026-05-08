// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RestApiKind = "RestApi"
)

var (
	RestApiGVK        = GroupVersion.WithKind(RestApiKind)
	RestApiAPIVersion = GroupVersion.String()
)

// RestApiSpec defines the desired state of RestApi
type RestApiSpec struct {
	// controllerRef is a reference to the Controller CR to which this has membership.
	// +required
	ControllerRef ObjectReference `json:"controllerRef"`

	// replicas is the desired number of replicas.
	// If unspecified, defaults to 1.
	// +optional
	// +default:=1
	Replicas *int32 `json:"replicas,omitempty"`

	// podDisruptionBudget configures a PodDisruptionBudget for the slurmrestd Deployment
	// when replicas is greater than one and enabled is true (the default).
	// +optional
	PodDisruptionBudget *RestApiPodDisruptionBudget `json:"podDisruptionBudget,omitempty"`

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

// RestApiPodDisruptionBudget configures voluntary disruption protection for the Rest API Deployment.
// It embeds policy/v1 PodDisruptionBudgetSpec so all standard PDB fields are supported (MinAvailable,
// MaxUnavailable, UnhealthyPodEvictionPolicy, etc.). The operator sets spec.selector to match the
// Rest API Deployment pods; any selector set here is overwritten during reconcile.
// Ref: https://kubernetes.io/docs/reference/kubernetes-api/policy-resources/pod-disruption-budget-v1/
type RestApiPodDisruptionBudget struct {
	// enabled controls whether a PodDisruptionBudget is reconciled while replicas is greater than one.
	// When false, no PodDisruptionBudget is created. When unset or true, a PDB is reconciled whenever replicas > 1.
	// +optional
	// +kubebuilder:default:=true
	Enabled *bool `json:"enabled,omitempty"`

	// +optional
	policyv1.PodDisruptionBudgetSpec `json:",inline"`
}

// RestApiStatus defines the observed state of Restapi
type RestApiStatus struct {
	// Represents the latest available observations of a Restapi's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
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
