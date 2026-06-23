// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// DeletionPolicy controls what happens to the Slurm entity when the CR is deleted.
// +kubebuilder:validation:Enum=Delete;Orphan
type DeletionPolicy string

const (
	DeletionPolicyDelete DeletionPolicy = "Delete"
	DeletionPolicyOrphan DeletionPolicy = "Orphan"
)

// AdminLevel is the Slurm administrator level granted to a user.
// +kubebuilder:validation:Enum=None;Operator;Administrator
type AdminLevel string

const (
	AdminLevelNone          AdminLevel = "None"
	AdminLevelOperator      AdminLevel = "Operator"
	AdminLevelAdministrator AdminLevel = "Administrator"
)

// AccountingLimits captures the common Slurm association limits/QoS/fairshare.
// Unset fields are not managed by the operator.
type AccountingLimits struct {
	// Fairshare is an integer number of shares, or the literal "parent".
	// +optional
	Fairshare *string `json:"fairshare,omitempty"`

	// Priority added to jobs using this association.
	// +optional
	Priority *int32 `json:"priority,omitempty"`

	// QOS is the list of QOS names allowed for this association.
	// +optional
	QOS []string `json:"qos,omitempty"`

	// DefaultQOS for this association; must be present in QOS.
	// +optional
	DefaultQOS *string `json:"defaultQos,omitempty"`

	// GrpTRES is the aggregate TRES limit, e.g. {"cpu":"10","gres/gpu":"4"}.
	// +optional
	GrpTRES map[string]string `json:"grpTRES,omitempty"`

	// +optional
	GrpJobs *int32 `json:"grpJobs,omitempty"`
	// +optional
	GrpSubmitJobs *int32 `json:"grpSubmitJobs,omitempty"`
	// +optional
	GrpWall *metav1.Duration `json:"grpWall,omitempty"`

	// +optional
	MaxJobs *int32 `json:"maxJobs,omitempty"`
	// +optional
	MaxSubmitJobs *int32 `json:"maxSubmitJobs,omitempty"`
	// MaxTRESPerJob is the per-job TRES limit, e.g. {"cpu":"4"}.
	// +optional
	MaxTRESPerJob map[string]string `json:"maxTRESPerJob,omitempty"`
	// +optional
	MaxWallPerJob *metav1.Duration `json:"maxWallPerJob,omitempty"`
}

// UserAssociation is a single account membership for a User.
type UserAssociation struct {
	// account name this user belongs to.
	// +required
	Account string `json:"account"`

	// partition scopes this association to a Slurm partition.
	// +optional
	Partition *string `json:"partition,omitempty"`

	// limits for this specific membership.
	// +optional
	Limits AccountingLimits `json:"limits,omitempty"`
}
