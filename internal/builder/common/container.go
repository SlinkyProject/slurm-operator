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

	clearBaseProbeHandler(opts.Base.LivenessProbe, opts.Merge.LivenessProbe)
	clearBaseProbeHandler(opts.Base.ReadinessProbe, opts.Merge.ReadinessProbe)
	clearBaseProbeHandler(opts.Base.StartupProbe, opts.Merge.StartupProbe)

	out := structutils.StrategicMergePatch(&opts.Base, &opts.Merge)
	return ptr.Deref(out, corev1.Container{})
}

// clearBaseProbeHandler clears the base probe's handler fields when the merge
// probe specifies a handler. ProbeHandler fields (Exec, HTTPGet, TCPSocket,
// GRPC) are mutually exclusive but strategic merge patch treats them as
// independent struct fields, so the base handler must be cleared to prevent
// both from appearing in the result.
func clearBaseProbeHandler(base, merge *corev1.Probe) {
	if !hasProbeHandler(base) || !hasProbeHandler(merge) {
		return
	}
	base.Exec = nil
	base.HTTPGet = nil
	base.TCPSocket = nil
	base.GRPC = nil
}

func hasProbeHandler(p *corev1.Probe) bool {
	return p != nil && (p.Exec != nil || p.HTTPGet != nil || p.TCPSocket != nil || p.GRPC != nil)
}
