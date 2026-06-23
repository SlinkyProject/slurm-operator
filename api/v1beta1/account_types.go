// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AccountKind = "Account"
)

var (
	AccountGVK        = GroupVersion.WithKind(AccountKind)
	AccountAPIVersion = GroupVersion.String()
)

// DeletionPolicy controls what happens to the Slurm entity when the CR is deleted.
// +kubebuilder:validation:Enum=Delete;Orphan
type DeletionPolicy string

const (
	DeletionPolicyDelete DeletionPolicy = "Delete"
	DeletionPolicyOrphan DeletionPolicy = "Orphan"
)

// AccountSpec defines the desired state of Account.
type AccountSpec struct {
	// controllerRef references the target Controller CR.
	// +required
	ControllerRef corev1.LocalObjectReference `json:"controllerRef"`

	// accountName in Slurm; defaults to metadata.name if empty.
	// +optional
	AccountName string `json:"accountName,omitempty"`

	// description of the account; defaults to accountName if empty.
	// +optional
	Description string `json:"description,omitempty"`

	// organization to which the account belongs; defaults to "root" if empty.
	// +optional
	Organization string `json:"organization,omitempty"`

	// parentAccount for hierarchy; defaults to "root".
	// +optional
	ParentAccount *string `json:"parentAccount,omitempty"`

	// deletionPolicy controls the Slurm-side entity on CR deletion.
	// +optional
	// +default:="Delete"
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`

	// limits applied to this account's association.
	// +optional
	Limits AssociationLimits `json:"limits,omitempty"`
}

// AccountStatus defines the observed state of Account.
type AccountStatus struct {
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=acct
// +kubebuilder:printcolumn:name="ACCOUNT",type="string",JSONPath=".spec.accountName",description="The Slurm account name."
// +kubebuilder:printcolumn:name="PARENT",type="string",JSONPath=".spec.parentAccount",description="The parent account."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Account is the Schema for the accounts API.
type Account struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AccountSpec   `json:"spec,omitempty"`
	Status AccountStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AccountList contains a list of Account.
type AccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Account `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Account{}, &AccountList{})
}
