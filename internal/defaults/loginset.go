// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// Default values for LoginSet Spec fields when unspecified.
const (
	DefaultLoginSetReplicas int32 = 1
)

func SetLoginSetDefaults(loginset *slinkyv1beta1.LoginSet) {
	if loginset == nil {
		return
	}
	s := &loginset.Spec

	if s.Replicas == nil {
		s.Replicas = ptr.To(DefaultLoginSetReplicas)
	}
}
