// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package workerbuilder

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
)

const (
	annotationDefaultContainer = "kubectl.kubernetes.io/default-container"
)

type WorkerBuilder struct {
	client        client.Client
	refResolver   *refresolver.RefResolver
	CommonBuilder common.CommonBuilder
}

func New(c client.Client) *WorkerBuilder {
	return &WorkerBuilder{
		client:        c,
		refResolver:   refresolver.New(c),
		CommonBuilder: *common.New(c),
	}
}
