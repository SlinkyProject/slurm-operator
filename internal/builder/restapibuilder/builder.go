// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapibuilder

import (
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	annotationDefaultContainer = "kubectl.kubernetes.io/default-container"
)

type RestapiBuilder struct {
	client        client.Client
	refResolver   *refresolver.RefResolver
	CommonBuilder common.CommonBuilder
}

func New(c client.Client) *RestapiBuilder {
	return &RestapiBuilder{
		client:        c,
		refResolver:   refresolver.New(c),
		CommonBuilder: *common.New(c),
	}
}
