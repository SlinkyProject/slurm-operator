// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
)

const (
	annotationDefaultContainer = "kubectl.kubernetes.io/default-container"
)

type ControllerBuilder struct {
	client        client.Client
	refResolver   *refresolver.RefResolver
	CommonBuilder common.CommonBuilder
}

func New(c client.Client) *ControllerBuilder {
	return &ControllerBuilder{
		client:        c,
		refResolver:   refresolver.New(c),
		CommonBuilder: *common.New(c),
	}
}
