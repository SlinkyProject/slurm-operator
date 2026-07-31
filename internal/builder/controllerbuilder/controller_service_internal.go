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

func (b *ControllerBuilder) BuildControllerServiceInternal(controller *slinkyv1beta1.Controller) (*corev1.Service, error) {
	opts := common.ServiceOpts{
		Key: controller.ServiceInternalKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: controller.Annotations,
			Labels:      structutils.MergeMaps(controller.Labels, labels.NewBuilder().WithControllerLabels(controller).Build()),
		},
		Selector: labels.NewBuilder().WithControllerSelectorLabels(controller).Build(),
		Headless: true,
	}

	port := corev1.ServicePort{
		Name:       labels.ControllerApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       common.SlurmctldPort,
		TargetPort: intstr.FromString(labels.ControllerApp),
	}
	opts.Ports = append(opts.Ports, port)

	return b.CommonBuilder.BuildService(opts, controller)
}
