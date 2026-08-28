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
	if opts.Merge.LivenessProbe != nil && opts.Base.LivenessProbe != nil {
		opts.Base.LivenessProbe.ProbeHandler = opts.Merge.LivenessProbe.ProbeHandler
	}
	if opts.Merge.ReadinessProbe != nil && opts.Base.ReadinessProbe != nil {
		opts.Base.ReadinessProbe.ProbeHandler = opts.Merge.ReadinessProbe.ProbeHandler
	}
	if opts.Merge.StartupProbe != nil && opts.Base.StartupProbe != nil {
		opts.Base.StartupProbe.ProbeHandler = opts.Merge.StartupProbe.ProbeHandler
	}
	// A lifecycle handler may only specify one action, so a merged handler has to
	// displace the base handler rather than be merged into it.
	if opts.Merge.Lifecycle != nil && opts.Base.Lifecycle != nil {
		if opts.Merge.Lifecycle.PostStart != nil {
			opts.Base.Lifecycle.PostStart = opts.Merge.Lifecycle.PostStart
		}
		if opts.Merge.Lifecycle.PreStop != nil {
			opts.Base.Lifecycle.PreStop = opts.Merge.Lifecycle.PreStop
		}
	}

	out := structutils.StrategicMergePatch(&opts.Base, &opts.Merge)
	return ptr.Deref(out, corev1.Container{})
}
