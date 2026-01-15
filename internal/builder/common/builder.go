// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnotationDefaultContainer = "kubectl.kubernetes.io/default-container"
)

type CommonBuilder struct {
	client      client.Client
	refResolver *refresolver.RefResolver
}

func New(c client.Client) *CommonBuilder {
	return &CommonBuilder{
		client:      c,
		refResolver: refresolver.New(c),
	}
}
