// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func (b *ControllerBuilder) BuildControllerService(controller *slinkyv1beta1.Controller) (*corev1.Service, error) {
	spec := controller.Spec.Service
	opts := common.ServiceOpts{
		Key: controller.ServiceKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: structutils.MergeMaps(controller.Annotations, controller.Spec.Service.Metadata.Annotations),
			Labels:      structutils.MergeMaps(controller.Labels, controller.Spec.Service.Metadata.Labels, labels.NewBuilder().WithControllerLabels(controller).Build()),
		},
		ServiceSpec: controller.Spec.Service.ServiceSpecWrapper.ServiceSpec,
		Selector: labels.NewBuilder().
			WithControllerSelectorLabels(controller).
			Build(),
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithControllerLabels(controller).Build())

	port := corev1.ServicePort{
		Name:       labels.ControllerApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       common.DefaultPort(int32(spec.Port), common.SlurmctldPort),
		TargetPort: intstr.FromString(labels.ControllerApp),
	}
	opts.Ports = append(opts.Ports, port)

	return b.CommonBuilder.BuildService(opts, controller)
}
