// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	UserKind = "User"
)

var (
	UserGVK        = GroupVersion.WithKind(UserKind)
	UserAPIVersion = GroupVersion.String()
)

// UserSpec defines the desired state of User.
// +kubebuilder:validation:XValidation:rule="self.associations.exists(a, a.account == self.defaultAccount)", message="defaultAccount must match one of the associations"
type UserSpec struct {
	// controllerRef references the target Controller CR.
	// +required
	ControllerRef corev1.LocalObjectReference `json:"controllerRef"`

	// userName in Slurm; defaults to metadata.name if empty.
	// +optional
	UserName string `json:"userName,omitempty"`

	// adminLevel granted to the user.
	// +optional
	// +default:="None"
	AdminLevel AdminLevel `json:"adminLevel,omitempty"`

	// defaultAccount must match one of the associations' account.
	// +required
	DefaultAccount string `json:"defaultAccount"`

	// associations are the account memberships for this user.
	// +required
	// +kubebuilder:validation:MinItems=1
	Associations []UserAssociation `json:"associations"`

	// deletionPolicy controls the Slurm-side entity on CR deletion.
	// +optional
	// +default:="Delete"
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// UserStatus defines the observed state of User.
type UserStatus struct {
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=slurmuser
// +kubebuilder:printcolumn:name="USER",type="string",JSONPath=".spec.userName",description="The Slurm user name."
// +kubebuilder:printcolumn:name="DEFAULT ACCOUNT",type="string",JSONPath=".spec.defaultAccount",description="The user's default account."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// User is the Schema for the users API.
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of User.
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
