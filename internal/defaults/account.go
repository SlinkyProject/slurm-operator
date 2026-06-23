// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

const DefaultParentAccount = "root"

// SetAccountDefaults applies defaults to an Account in place.
func SetAccountDefaults(a *slinkyv1beta1.Account) {
	if a.Spec.AccountName == "" {
		a.Spec.AccountName = a.Name
	}
	if a.Spec.ParentAccount == nil {
		a.Spec.ParentAccount = ptr.To(DefaultParentAccount)
	}
	if a.Spec.DeletionPolicy == "" {
		a.Spec.DeletionPolicy = slinkyv1beta1.DeletionPolicyDelete
	}
}
