// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accountingbuilder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	common "github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func (b *AccountingBuilder) BuildAccountingService(accounting *slinkyv1beta1.Accounting) (*corev1.Service, error) {
	spec := accounting.Spec.Service
	opts := common.ServiceOpts{
		Key: accounting.ServiceKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: structutils.MergeMaps(accounting.Annotations, accounting.Spec.Service.Metadata.Annotations),
			Labels:      structutils.MergeMaps(accounting.Labels, accounting.Spec.Service.Metadata.Labels, labels.NewBuilder().WithAccountingLabels(accounting).Build()),
		},
		ServiceSpec: accounting.Spec.Service.ServiceSpecWrapper.ServiceSpec,
		Selector: labels.NewBuilder().
			WithAccountingSelectorLabels(accounting).
			Build(),
	}

	port := corev1.ServicePort{
		Name:       labels.AccountingApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       common.DefaultPort(int32(spec.Port), common.SlurmdbdPort),
		TargetPort: intstr.FromString(labels.AccountingApp),
	}
	opts.Ports = append(opts.Ports, port)

	return b.CommonBuilder.BuildService(opts, accounting)
}
