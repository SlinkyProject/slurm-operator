// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ObjectReference is a reference to an object.
// +structType=atomic
type ObjectReference struct {
	// Namespace of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// +optional
	Name string `json:"name,omitempty"`
}

func (o *ObjectReference) NamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Name,
		Namespace: o.Namespace,
	}
}

func (o *ObjectReference) IsMatch(key types.NamespacedName) bool {
	switch {
	case o.Name != key.Name:
		return false
	case o.Namespace != key.Namespace:
		return false
	default:
		return true
	}
}

type JwtSecretKeySelector struct {
	// SecretKeySelector selects a key of a Secret.
	// +structType=atomic
	corev1.SecretKeySelector `json:",inline"`

	// The namespace of the Slurm `auth/jwt` JWT HS256 key.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// PodTemplate describes a template for creating copies of a predefined pod.
type PodTemplate struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	PodMetadata Metadata `json:"metadata,omitempty"`

	// PodSpec is a description of a pod.
	// +optional
	PodSpecWrapper PodSpecWrapper `json:"spec,omitempty"`
}

// Metadata defines the metadata to added to resources.
type Metadata struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PodSpecWrapper is a wrapper around corev1.PodSpec with a custom implementation
// of MarshalJSON and UnmarshalJSON which delegate to the underlying Spec to avoid CRD pollution.
// +kubebuilder:pruning:PreserveUnknownFields
type PodSpecWrapper struct {
	corev1.PodSpec `json:"-"`
}

// MarshalJSON defers JSON encoding data from the wrapper.
func (o *PodSpecWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.PodSpec)
}

// UnmarshalJSON will decode the data into the wrapper.
func (o *PodSpecWrapper) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &o.PodSpec)
}

func (o *PodSpecWrapper) DeepCopy() *PodSpecWrapper {
	return &PodSpecWrapper{
		PodSpec: o.PodSpec,
	}
}

// ContainerWrapper is a wrapper around corev1.Container with a custom implementation
// of MarshalJSON and UnmarshalJSON which delegate to the underlying Spec to avoid CRD pollution.
// +kubebuilder:pruning:PreserveUnknownFields
type ContainerWrapper struct {
	corev1.Container `json:"-"`
}

// MarshalJSON defers JSON encoding data from the wrapper.
func (o *ContainerWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.Container)
}

// UnmarshalJSON will decode the data into the wrapper.
func (o *ContainerWrapper) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &o.Container)
}

func (o *ContainerWrapper) DeepCopy() *ContainerWrapper {
	return &ContainerWrapper{
		Container: o.Container,
	}
}

// ServiceSpec defines a template to customize Service objects.
type ServiceSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	Metadata Metadata `json:"metadata,omitempty"`

	// ServiceSpec describes the attributes that a user creates on a service.
	// +optional
	ServiceSpecWrapper ServiceSpecWrapper `json:"spec,omitempty"`

	// The external service port number.
	// +optional
	Port int `json:"port"`

	// The port on each node on which this service is exposed when type is
	// NodePort or LoadBalancer.  Usually assigned by the system. If a value is
	// specified, in-range, and not in use it will be used, otherwise the
	// operation will fail.  If not specified, a port will be allocated if this
	// Service requires one.  If this field is specified when creating a
	// Service which does not need it, creation will fail.
	// +optional
	NodePort int `json:"nodePort,omitempty"`
}

// ServiceSpecWrapper is a wrapper around corev1.Container with a custom implementation
// of MarshalJSON and UnmarshalJSON which delegate to the underlying Spec to avoid CRD pollution.
// +kubebuilder:pruning:PreserveUnknownFields
type ServiceSpecWrapper struct {
	corev1.ServiceSpec `json:"-"`
}

// MarshalJSON defers JSON encoding data from the wrapper.
func (o *ServiceSpecWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.ServiceSpec)
}

// UnmarshalJSON will decode the data into the wrapper.
func (o *ServiceSpecWrapper) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &o.ServiceSpec)
}

func (o *ServiceSpecWrapper) DeepCopy() *ServiceSpecWrapper {
	return &ServiceSpecWrapper{
		ServiceSpec: o.ServiceSpec,
	}
}
