// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

import (
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// restapiPodDisruptionBudgetDesired reports whether the RestApi controller should reconcile a PDB.
// A PDB is desired when replicas > 1 and PodDisruptionBudget is not explicitly disabled (enabled defaults to true).
func restapiPodDisruptionBudgetDesired(restapi *slinkyv1beta1.RestApi) bool {
	if ptr.Deref(restapi.Spec.Replicas, 1) <= 1 {
		return false
	}
	cfg := restapi.Spec.PodDisruptionBudget
	if cfg != nil && cfg.Enabled != nil && !*cfg.Enabled {
		return false
	}
	return true
}
