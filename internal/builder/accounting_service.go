// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func (b *Builder) BuildAccountingService(accounting *slinkyv1alpha1.Accounting) (*corev1.Service, error) {
	spec := accounting.Spec.Service
	opts := ServiceOpts{
		Key:         accounting.ServiceKey(),
		Metadata:    accounting.Spec.Template.PodMetadata,
		ServiceSpec: accounting.Spec.Service.ServiceSpec,
		Selector: labels.NewBuilder().
			WithAccountingSelectorLabels(accounting).
			Build(),
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithAccountingLabels(accounting).Build())

	port := corev1.ServicePort{
		Name:       labels.AccountingApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       defaultPort(int32(spec.Port), SlurmdbdPort),
		TargetPort: intstr.FromString(labels.AccountingApp),
	}
	opts.Ports = append(opts.Ports, port)

	return b.BuildService(opts, accounting)
}
