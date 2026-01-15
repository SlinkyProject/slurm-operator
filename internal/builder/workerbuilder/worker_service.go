// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package workerbuilder

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
)

// BuildClusterWorkerService creates a single headless service for ALL worker NodeSets in the same Slurm cluster
func (b *WorkerBuilder) BuildClusterWorkerService(nodeset *slinkyv1beta1.NodeSet) (*corev1.Service, error) {
	selectorLabels := labels.NewBuilder().
		WithApp(labels.WorkerApp).
		WithCluster(nodeset.Spec.ControllerRef.Name).
		Build()

	opts := common.ServiceOpts{
		Key: types.NamespacedName{
			Name:      common.SlurmClusterWorkerServiceName(nodeset.Spec.ControllerRef.Name),
			Namespace: nodeset.Namespace,
		},
		Selector: selectorLabels,
		Headless: true,
	}

	port := corev1.ServicePort{
		Name:       labels.WorkerApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       common.SlurmdPort,
		TargetPort: intstr.FromString(labels.WorkerApp),
	}
	opts.Ports = append(opts.Ports, port)

	out, err := b.CommonBuilder.BuildService(opts, nodeset)
	if err != nil {
		return nil, err
	}

	// No NodeSet should be the controller of this service
	if err := controllerutil.RemoveControllerReference(nodeset, out, b.client.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to remove owner controller: %w", err)
	}

	return out, nil
}
