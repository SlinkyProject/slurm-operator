// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapibuilder

import (
	"fmt"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
)

// RestapiPodDisruptionBudgetName returns the PodDisruptionBudget name for a RestApi Deployment.
func RestapiPodDisruptionBudgetName(restapi *slinkyv1beta1.RestApi) string {
	return fmt.Sprintf("%s-pdb", restapi.Key().Name)
}

// BuildRestapiPodDisruptionBudget builds a PDB that matches the RestApi Deployment pod selector.
// Callers should only reconcile it when spec.replicas is greater than one and PDB is desired.
func (b *RestapiBuilder) BuildRestapiPodDisruptionBudget(restapi *slinkyv1beta1.RestApi) (*policyv1.PodDisruptionBudget, error) {
	key := restapi.Key()
	selectorLabels := labels.NewBuilder().WithRestapiSelectorLabels(restapi).Build()

	pdbSpec := policyv1.PodDisruptionBudgetSpec{}
	if restapi.Spec.PodDisruptionBudget != nil {
		restapi.Spec.PodDisruptionBudget.PodDisruptionBudgetSpec.DeepCopyInto(&pdbSpec)
	}
	if pdbSpec.MinAvailable == nil && pdbSpec.MaxUnavailable == nil {
		pdbSpec.MinAvailable = ptr.To(intstr.FromInt32(1))
	}
	pdbSpec.Selector = &metav1.LabelSelector{
		MatchLabels: selectorLabels,
	}

	opts := common.PodDisruptionBudgetOpts{
		Key: types.NamespacedName{
			Name:      RestapiPodDisruptionBudgetName(restapi),
			Namespace: key.Namespace,
		},
		PodDisruptionBudgetSpec: pdbSpec,
	}
	return b.CommonBuilder.BuildPodDisruptionBudget(opts, restapi)
}
