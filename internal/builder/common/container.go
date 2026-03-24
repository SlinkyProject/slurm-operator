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
	// Clear mutually exclusive probe handlers during container merge.
	// When the user provides a probe with a different handler type (e.g., exec)
	// than the base (e.g., tcpSocket), the base handler must be cleared first.
	// StrategicMergePatch merges both handlers into the same probe, producing
	// an invalid spec with multiple handler types:
	//   "may not specify more than 1 handler type"
	// Replacing just the ProbeHandler preserves the base probe's settings
	// (periodSeconds, failureThreshold, etc.) while swapping the handler.
	if opts.Merge.LivenessProbe != nil {
		if opts.Base.LivenessProbe != nil {
			opts.Base.LivenessProbe.ProbeHandler = opts.Merge.LivenessProbe.ProbeHandler
		} else {
			opts.Base.LivenessProbe = opts.Merge.LivenessProbe
		}
	}
	if opts.Merge.ReadinessProbe != nil {
		if opts.Base.ReadinessProbe != nil {
			opts.Base.ReadinessProbe.ProbeHandler = opts.Merge.ReadinessProbe.ProbeHandler
		} else {
			opts.Base.ReadinessProbe = opts.Merge.ReadinessProbe
		}
	}
	if opts.Merge.StartupProbe != nil {
		if opts.Base.StartupProbe != nil {
			opts.Base.StartupProbe.ProbeHandler = opts.Merge.StartupProbe.ProbeHandler
		} else {
			opts.Base.StartupProbe = opts.Merge.StartupProbe
		}
	}

	out := structutils.StrategicMergePatch(&opts.Base, &opts.Merge)
	return ptr.Deref(out, corev1.Container{})
}
