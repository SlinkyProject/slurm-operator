// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// Default values for Controller Spec fields when unspecified.
const (
	DefaultControllerPersistenceEnabled bool = true
)

func SetControllerDefaults(controller *slinkyv1beta1.Controller) {
	if controller == nil {
		return
	}
	s := &controller.Spec

	if s.Persistence.Enabled == nil {
		s.Persistence.Enabled = ptr.To(DefaultControllerPersistenceEnabled)
	}
}
