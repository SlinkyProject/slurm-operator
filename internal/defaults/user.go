// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"

// SetUserDefaults applies defaults to a User in place.
func SetUserDefaults(u *slinkyv1beta1.User) {
	if u.Spec.UserName == "" {
		u.Spec.UserName = u.Name
	}
	if u.Spec.AdminLevel == "" {
		u.Spec.AdminLevel = slinkyv1beta1.AdminLevelNone
	}
	if u.Spec.DeletionPolicy == "" {
		u.Spec.DeletionPolicy = slinkyv1beta1.DeletionPolicyDelete
	}
}
