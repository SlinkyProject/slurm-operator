// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/builder/labels"
	"github.com/togethercomputer/slurm-operator/internal/utils/structutils"
)

func (b *Builder) BuildControllerService(controller *slinkyv1beta1.Controller) (*corev1.Service, error) {
	spec := controller.Spec.Service
	opts := ServiceOpts{
		Key:         controller.ServiceKey(),
		Metadata:    controller.Spec.Service.Metadata,
		ServiceSpec: controller.Spec.Service.ServiceSpecWrapper.ServiceSpec,
		Selector: labels.NewBuilder().
			WithControllerSelectorLabels(controller).
			Build(),
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithControllerLabels(controller).Build())

	port := corev1.ServicePort{
		Name:       labels.ControllerApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       defaultPort(int32(spec.Port), SlurmctldPort),
		TargetPort: intstr.FromString(labels.ControllerApp),
	}
	opts.Ports = append(opts.Ports, port)

	return b.BuildService(opts, controller)
}
