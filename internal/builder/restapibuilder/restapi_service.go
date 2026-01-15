// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapibuilder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func (b *RestapiBuilder) BuildRestapiService(restapi *slinkyv1beta1.RestApi) (*corev1.Service, error) {
	spec := restapi.Spec.Service
	opts := common.ServiceOpts{
		Key:         restapi.ServiceKey(),
		Metadata:    restapi.Spec.Service.Metadata,
		ServiceSpec: restapi.Spec.Service.ServiceSpecWrapper.ServiceSpec,
		Selector: labels.NewBuilder().
			WithRestapiSelectorLabels(restapi).
			Build(),
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithRestapiLabels(restapi).Build())

	port := corev1.ServicePort{
		Name:       labels.RestapiApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       common.DefaultPort(int32(spec.Port), SlurmrestdPort),
		TargetPort: intstr.FromString(labels.RestapiApp),
	}
	opts.Ports = append(opts.Ports, port)

	return b.CommonBuilder.BuildService(opts, restapi)
}
