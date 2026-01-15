// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

type ContainerOpts struct {
	Base  corev1.Container
	Merge corev1.Container
}

func (b *CommonBuilder) BuildContainer(opts ContainerOpts) corev1.Container {
	// Handle non `patchStrategy=merge` fields as if they were.

	opts.Base.Args = structutils.MergeList(opts.Base.Args, opts.Merge.Args)
	opts.Merge.Args = []string{}

	out := structutils.StrategicMergePatch(&opts.Base, &opts.Merge)
	return ptr.Deref(out, corev1.Container{})
}
