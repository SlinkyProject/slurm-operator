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

func (b *Builder) BuildLoginService(loginset *slinkyv1beta1.LoginSet) (*corev1.Service, error) {
	spec := loginset.Spec.Service
	opts := ServiceOpts{
		Key:         loginset.ServiceKey(),
		Metadata:    loginset.Spec.Service.Metadata,
		ServiceSpec: loginset.Spec.Service.ServiceSpecWrapper.ServiceSpec,
		Selector: labels.NewBuilder().
			WithLoginSelectorLabels(loginset).
			Build(),
	}

	opts.Metadata.Labels = structutils.MergeMaps(opts.Metadata.Labels, labels.NewBuilder().WithLoginLabels(loginset).Build())

	port := corev1.ServicePort{
		Name:       labels.LoginApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       defaultPort(int32(spec.Port), LoginPort),
		TargetPort: intstr.FromString(labels.LoginApp),
		NodePort:   int32(spec.NodePort),
	}
	opts.Ports = append(opts.Ports, port)

	return b.BuildService(opts, loginset)
}
