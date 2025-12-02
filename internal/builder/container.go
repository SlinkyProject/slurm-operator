// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

type ContainerOpts struct {
	base  corev1.Container
	merge corev1.Container
}

func (b *Builder) BuildContainer(opts ContainerOpts) corev1.Container {
	// Handle non `patchStrategy=merge` fields as if they were.
	opts.base.Args = structutils.MergeList(opts.base.Args, opts.merge.Args)
	opts.merge.Args = []string{}

	out := structutils.StrategicMergePatch(&opts.base, &opts.merge)
	return ptr.Deref(out, corev1.Container{})
}
