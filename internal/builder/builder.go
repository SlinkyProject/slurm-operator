// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
)

const (
	annotationDefaultContainer = "kubectl.kubernetes.io/default-container"
)

type Builder struct {
	client      client.Client
	refResolver *refresolver.RefResolver
}

func New(c client.Client) *Builder {
	return &Builder{
		client:      c,
		refResolver: refresolver.New(c),
	}
}
