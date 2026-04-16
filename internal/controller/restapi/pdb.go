// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

import (
	"k8s.io/apimachinery/pkg/util/intstr"
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

// restapiPodDisruptionBudgetMinDemandsAllReplicas reports whether spec.podDisruptionBudget.minAvailable
// requires every replica to stay available (same as replica count, or 100%), which prevents voluntary
// disruption of any pod. The controller skips reconciling a PDB in that case and emits a warning event.
func restapiPodDisruptionBudgetMinDemandsAllReplicas(restapi *slinkyv1beta1.RestApi) bool {
	replicas := ptr.Deref(restapi.Spec.Replicas, 1)
	if replicas < 2 {
		return false
	}
	cfg := restapi.Spec.PodDisruptionBudget
	if cfg == nil {
		return false
	}
	ma := cfg.MinAvailable
	if ma == nil {
		return false
	}
	switch ma.Type {
	case intstr.Int:
		return ma.IntVal == replicas
	case intstr.String:
		v, err := intstr.GetScaledValueFromIntOrPercent(ma, int(replicas), true)
		if err != nil {
			return false
		}
		return v == int(replicas)
	default:
		return false
	}
}
