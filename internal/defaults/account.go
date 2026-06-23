// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

const (
	DefaultParentAccount = "root"
	DefaultOrganization  = "root"
)

// SetAccountDefaults applies defaults to an Account in place.
func SetAccountDefaults(a *slinkyv1beta1.Account) {
	if a.Spec.AccountName == "" {
		a.Spec.AccountName = a.Name
	}
	// slurmdbd rejects accounts with an empty description or organization, so
	// default the description to the account name and the organization to root.
	if a.Spec.Description == "" {
		a.Spec.Description = a.Spec.AccountName
	}
	if a.Spec.Organization == "" {
		a.Spec.Organization = DefaultOrganization
	}
	if a.Spec.ParentAccount == nil {
		a.Spec.ParentAccount = ptr.To(DefaultParentAccount)
	}
	if a.Spec.DeletionPolicy == "" {
		a.Spec.DeletionPolicy = slinkyv1beta1.DeletionPolicyDelete
	}
}
