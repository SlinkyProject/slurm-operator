// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
)

type PodDisruptionBudgetOpts struct {
	Key      types.NamespacedName
	Metadata slinkyv1beta1.Metadata
	policyv1.PodDisruptionBudgetSpec
}

func (b *CommonBuilder) BuildPodDisruptionBudget(opts PodDisruptionBudgetOpts, owner metav1.Object) (*policyv1.PodDisruptionBudget, error) {
	objectMeta := metadata.NewBuilder(opts.Key).
		WithMetadata(opts.Metadata).
		Build()

	out := &policyv1.PodDisruptionBudget{
		ObjectMeta: objectMeta,
		Spec:       opts.PodDisruptionBudgetSpec,
	}

	if err := controllerutil.SetControllerReference(owner, out, b.client.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to set owner controller: %w", err)
	}

	return out, nil
}
