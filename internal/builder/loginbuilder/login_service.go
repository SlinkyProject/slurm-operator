// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func (b *LoginBuilder) BuildLoginService(loginset *slinkyv1beta1.LoginSet) (*corev1.Service, error) {
	spec := loginset.Spec.Service
	opts := common.ServiceOpts{
		Key: loginset.ServiceKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: structutils.MergeMaps(loginset.Annotations, loginset.Spec.Service.Metadata.Annotations),
			Labels:      structutils.MergeMaps(loginset.Labels, loginset.Spec.Service.Metadata.Labels, labels.NewBuilder().WithLoginLabels(loginset).Build()),
		},
		ServiceSpec: loginset.Spec.Service.ServiceSpecWrapper.ServiceSpec,
		Selector: labels.NewBuilder().
			WithLoginSelectorLabels(loginset).
			Build(),
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithLoginLabels(loginset).Build())

	port := corev1.ServicePort{
		Name:       labels.LoginApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       common.DefaultPort(int32(spec.Port), LoginPort),
		TargetPort: intstr.FromString(labels.LoginApp),
		NodePort:   int32(spec.NodePort),
	}
	opts.Ports = append(opts.Ports, port)

	return b.CommonBuilder.BuildService(opts, loginset)
}
