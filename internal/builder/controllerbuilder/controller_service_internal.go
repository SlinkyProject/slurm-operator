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
	spec := controller.Spec.Service
	opts := common.ServiceOpts{
		Key: controller.ServiceInternalKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: structutils.MergeMaps(controller.Annotations, spec.Metadata.Annotations),
			Labels:      structutils.MergeMaps(controller.Labels, spec.Metadata.Labels, labels.NewBuilder().WithControllerLabels(controller).Build()),
		},
		ServiceSpec: spec.ServiceSpecWrapper.ServiceSpec,
		Selector: labels.NewBuilder().
			WithControllerSelectorLabels(controller).
			Build(),
		Headless: true,
	}

	port := corev1.ServicePort{
		Name:       labels.ControllerApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       common.DefaultPort(int32(spec.Port), common.SlurmctldPort),
		TargetPort: intstr.FromString(labels.ControllerApp),
		NodePort:   int32(spec.NodePort),
	}
	opts.Ports = append(opts.Ports, port)

	return b.CommonBuilder.BuildService(opts, controller)
}
