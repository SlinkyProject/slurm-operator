// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package workerbuilder

import (
	"fmt"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
)

// BuildClusterWorkerPodDisruptionBudget creates a single PodDisruptionBudget for ALL worker NodeSets in the same Slurm cluster
// The PodDisruptionBudget name is derived from the Slurm cluster name to support hybrid deployments
func (b *WorkerBuilder) BuildClusterWorkerPodDisruptionBudget(nodeset *slinkyv1beta1.NodeSet) (*policyv1.PodDisruptionBudget, error) {
	selectorLabels := labels.NewBuilder().
		WithPodProtect().WithCluster(nodeset.Spec.ControllerRef.Name).Build()
	opts := common.PodDisruptionBudgetOpts{
		Key: types.NamespacedName{
			Name:      common.SlurmClusterWorkerPodDisruptionBudgetName(nodeset.Spec.ControllerRef.Name),
			Namespace: nodeset.Namespace,
		},
		PodDisruptionBudgetSpec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			MaxUnavailable: ptr.To(intstr.FromInt(0)),
		},
	}

	out, err := b.CommonBuilder.BuildPodDisruptionBudget(opts, nodeset)
	if err != nil {
		return nil, err
	}

	// No NodeSet should be the controller of this
	if err := controllerutil.RemoveControllerReference(nodeset, out, b.client.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to remove owner controller: %w", err)
	}

	return out, nil
}
