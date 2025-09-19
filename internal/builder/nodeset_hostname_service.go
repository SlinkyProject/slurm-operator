// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

// BuildClusterWideWorkerService creates a single headless service for ALL worker NodeSets in the same Slurm cluster
// The service name is derived from the Slurm cluster name to support hybrid deployments
func (b *Builder) BuildClusterWideWorkerService(nodeset *slinkyv1alpha1.NodeSet) (*corev1.Service, error) {
	// Service name based on Slurm cluster (ControllerRef.Name)
	serviceName := slurmClusterWorkerServiceName(nodeset.Spec.ControllerRef.Name)

	opts := ServiceOpts{
		Key: types.NamespacedName{
			Name:      serviceName,
			Namespace: nodeset.Namespace,
		},
		Metadata:    slinkyv1alpha1.Metadata{},
		ServiceSpec: corev1.ServiceSpec{},
		Selector: labels.NewBuilder().
			WithApp(labels.WorkerApp).
			WithController(nodeset.Spec.ControllerRef.Name).
			Build(),
		Headless: true,
	}

	// Add labels
	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels,
		labels.NewBuilder().WithWorkerLabels(nodeset).Build())

	// Add slurmd port
	port := corev1.ServicePort{
		Name:       labels.WorkerApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       SlurmdPort,
		TargetPort: intstr.FromString(labels.WorkerApp),
	}
	opts.Ports = append(opts.Ports, port)

	// Build service with NodeSet
	return b.BuildService(opts, nodeset)
}
