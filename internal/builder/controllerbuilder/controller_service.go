// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

// BuildControllerService builds the controller Service. Its name feeds the
// immutable StatefulSet.spec.serviceName, so it is the same in both modes.
//
// With a single replica it honors user `Spec.Service` overrides, matching
// pre-HA behavior. With Replicas>1 it is the headless governing Service that
// provides the stable per-pod DNS (`<pod>.<svc>.<ns>`) required for Slurm
// active/passive HA, with publishNotReadyAddresses so a standby backup stays
// addressable; user overrides are rejected (port/nodePort also by CEL on
// ControllerSpec).
func (b *ControllerBuilder) BuildControllerService(controller *slinkyv1beta1.Controller) (*corev1.Service, error) {
	spec := controller.Spec.Service
	opts := common.ServiceOpts{
		Key: controller.ServiceKey(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: structutils.MergeMaps(controller.Annotations, spec.Metadata.Annotations),
			Labels:      structutils.MergeMaps(controller.Labels, spec.Metadata.Labels, labels.NewBuilder().WithControllerLabels(controller).Build()),
		},
		Selector: labels.NewBuilder().
			WithControllerSelectorLabels(controller).
			Build(),
	}

	port := corev1.ServicePort{
		Name:       labels.ControllerApp,
		Protocol:   corev1.ProtocolTCP,
		Port:       common.SlurmctldPort,
		TargetPort: intstr.FromString(labels.ControllerApp),
	}

	if controller.Replicas() > 1 {
		var overrides []string
		if spec.Port != 0 {
			overrides = append(overrides, "port")
		}
		if spec.NodePort != 0 {
			overrides = append(overrides, "nodePort")
		}
		if !apiequality.Semantic.DeepEqual(spec.ServiceSpecWrapper.ServiceSpec, corev1.ServiceSpec{}) {
			overrides = append(overrides, "spec")
		}
		if len(overrides) > 0 {
			return nil, fmt.Errorf("spec.service overrides (%s) cannot be combined with replicas > 1: the governing Service must remain headless for per-pod DNS", strings.Join(overrides, ", "))
		}
		opts.Headless = true
	} else {
		opts.ServiceSpec = spec.ServiceSpecWrapper.ServiceSpec
		port.Port = common.DefaultPort(int32(spec.Port), common.SlurmctldPort)
		port.NodePort = int32(spec.NodePort)
	}
	opts.Ports = append(opts.Ports, port)

	return b.CommonBuilder.BuildService(opts, controller)
}
