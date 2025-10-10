// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AccountingKind = "Accounting"
)

var (
	AccountingGVK        = GroupVersion.WithKind(AccountingKind)
	AccountingAPIVersion = GroupVersion.String()
)

// AccountingSpec defines the desired state of Accounting
type AccountingSpec struct {
	// Slurm `auth/slurm` key authentication.
	// +required
	SlurmKeyRef corev1.SecretKeySelector `json:"slurmKeyRef,omitzero"`

	// Slurm `auth/jwt` JWT HS256 key authentication.
	// +required
	JwtHs256KeyRef corev1.SecretKeySelector `json:"jwtHs256KeyRef,omitzero"`

	// The slurmdbd container configuration.
	// See corev1.Container spec.
	// Ref: https://github.com/kubernetes/api/blob/master/core/v1/types.go#L2885
	// +optional
	Slurmdbd ContainerWrapper `json:"slurmdbd,omitempty"`

	// Template is the object that describes the pod that will be created if
	// insufficient replicas are detected.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#pod-template
	// +optional
	Template PodTemplate `json:"template,omitempty"`

	// StorageConfig is the configuration for mysql/mariadb access.
	// +optional
	StorageConfig StorageConfig `json:"storageConfig,omitzero"`

	// ExtraConf is appended onto the end of the `slurmdbd.conf` file.
	// Ref: https://slurm.schedmd.com/slurmdbd.conf.html
	// +optional
	ExtraConf string `json:"extraConf,omitzero"`

	// Service defines a template for a Kubernetes Service object.
	// +optional
	Service ServiceSpec `json:"service,omitzero"`
}

// StorageConfig defines access to mysql/mariadb.
type StorageConfig struct {
	// Define the name of the host the database is running where we are going to
	// store the data.
	// Ref: https://slurm.schedmd.com/slurmdbd.conf.html#OPT_StorageHost
	// +required
	Host string `json:"host,omitzero"`

	// The port number that the Slurm Database Daemon (slurmdbd) communicates
	// with the database.
	// Default is 3306.
	// Ref: https://slurm.schedmd.com/slurmdbd.conf.html#OPT_StoragePort
	// +optional
	// +default:=3306
	Port int `json:"port,omitzero"`

	// Specify the name of the database as the location where accounting records
	// are written.
	// Default is "slurm_acct_db".
	// Ref: https://slurm.schedmd.com/slurmdbd.conf.html#OPT_StorageLoc
	// +optional
	// +default:="slurm_acct_db"
	Database string `json:"database,omitzero"`

	// Define the name of the user we are going to connect to the database with
	// to store the job accounting data.
	// Ref: https://slurm.schedmd.com/slurmdbd.conf.html#OPT_StorageUser
	// +optional
	Username string `json:"username,omitzero"`

	// PasswordKeyRef is a reference to a secret containing the password for the
	// user, specified by username, to access the given database.
	// Ref: https://slurm.schedmd.com/slurmdbd.conf.html#OPT_StoragePass
	// +required
	PasswordKeyRef corev1.SecretKeySelector `json:"passwordKeyRef,omitzero"`
}

// AccountingStatus defines the observed state of Accounting
type AccountingStatus struct {
	// Represents the latest available observations of a Accounting's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=slurmdbd
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Accounting is the Schema for the accountings API
type Accounting struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AccountingSpec   `json:"spec,omitempty"`
	Status AccountingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AccountingList contains a list of Accounting
type AccountingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Accounting `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Accounting{}, &AccountingList{})
}
