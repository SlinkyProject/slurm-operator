// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ControllerKind = "Controller"
)

var (
	ControllerGVK        = GroupVersion.WithKind(ControllerKind)
	ControllerAPIVersion = GroupVersion.String()
)

// ControllerSpec defines the desired state of Controller
type ControllerSpec struct {
	// The Slurm ClusterName, which uniquely identifies the Slurm Cluster to
	// itself and accounting.
	// Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_ClusterName
	// +optional
	ClusterName string `json:"clusterName,omitzero"`

	// Slurm `auth/slurm` key authentication.
	// +required
	SlurmKeyRef corev1.SecretKeySelector `json:"slurmKeyRef,omitzero"`

	// Slurm `auth/jwt` JWT HS256 key authentication.
	// +required
	JwtHs256KeyRef corev1.SecretKeySelector `json:"jwtHs256KeyRef,omitzero"`

	// accountingRef is a reference to the Accounting CR to which this has membership.
	// +optional
	AccountingRef ObjectReference `json:"accountingRef"`

	// The slurmctld container configuration.
	// See corev1.Container spec.
	// Ref: https://github.com/kubernetes/api/blob/master/core/v1/types.go#L2885
	// +optional
	Slurmctld ContainerWrapper `json:"slurmctld,omitempty"`

	// The reconfigure container configuration.
	// +optional
	Reconfigure ContainerWrapper `json:"reconfigure,omitzero"`

	// The logfile sidecar configuration.
	// +optional
	LogFile ContainerWrapper `json:"logfile,omitzero"`

	// Template is the object that describes the pod that will be created if
	// insufficient replicas are detected.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#pod-template
	// +optional
	Template PodTemplate `json:"template,omitempty"`

	// ExtraConf is appended onto the end of the `slurm.conf` file.
	// Ref: https://slurm.schedmd.com/slurm.conf.html
	// +optional
	ExtraConf string `json:"extraConf,omitempty"`

	// ConfigFileRefs is a list of ConfigMap references containing files to be mounted in `/etc/slurm`.
	// Ref: https://slurm.schedmd.com/slurm.conf.html
	// +optional
	ConfigFileRefs []ObjectReference `json:"configFileRefs,omitzero"`

	// PrologScriptRefs is a list of prolog scripts to be mounted in `/etc/slurm`.
	// Ref: https://slurm.schedmd.com/prolog_epilog.html
	// +optional
	PrologScriptRefs []ObjectReference `json:"prologScriptRefs,omitzero"`

	// EpilogScriptRefs is a list of epilog scripts to be mounted in `/etc/slurm`.
	// Ref: https://slurm.schedmd.com/prolog_epilog.html
	// +optional
	EpilogScriptRefs []ObjectReference `json:"epilogScriptRefs,omitzero"`

	// PrologSlurmctldScriptRefs is a list of PrologSlurmctld scripts to be mounted in `/etc/slurm`.
	// Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_PrologSlurmctld
	// +optional
	PrologSlurmctldScriptRefs []ObjectReference `json:"prologSlurmctldScriptRefs,omitzero"`

	// EpilogSlurmctldScriptRefs is a list of EpilogSlurmctld scripts to be mounted in `/etc/slurm`.
	// Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_EpilogSlurmctld
	// +optional
	EpilogSlurmctldScriptRefs []ObjectReference `json:"epilogSlurmctldScriptRefs,omitzero"`

	// Persistence defines a persistent volume for the slurm controller to store its save-state.
	// Used to recover from system failures or from pod upgrades.
	// +optional
	Persistence ControllerPersistence `json:"persistence,omitzero"`

	// Service defines a template for a Kubernetes Service object.
	// +optional
	Service ServiceSpec `json:"service,omitzero"`
}

type ControllerPersistence struct {
	// Enabled controls if the optional accounting subsystem is enabled.
	// +default:=true
	Enabled bool `json:"enabled"`

	// ExistingClaim is the name of an existing `PersistentVolumeClaim` to use instead.
	// If this is not empty, then certain other fields will be ignored.
	// +optional
	ExistingClaim string `json:"existingClaim,omitempty"`

	// +optional
	corev1.PersistentVolumeClaimSpec `json:",inline"`
}

// ControllerStatus defines the observed state of Controller
type ControllerStatus struct {
	// Represents the latest available observations of a Controller's current state.
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
// +kubebuilder:resource:shortName=slurmctld
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Controller is the Schema for the controllers API
type Controller struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControllerSpec   `json:"spec,omitempty"`
	Status ControllerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ControllerList contains a list of Controller
type ControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Controller `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Controller{}, &ControllerList{})
}
