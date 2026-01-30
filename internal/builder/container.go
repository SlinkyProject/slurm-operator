// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/togethercomputer/slurm-operator/internal/utils/reflectutils"
	"github.com/togethercomputer/slurm-operator/internal/utils/structutils"
)

type ContainerOpts struct {
	base  corev1.Container
	merge corev1.Container
}

func (b *Builder) BuildContainer(opts ContainerOpts) corev1.Container {
	out := opts.base

	out.Name = reflectutils.UseNonZeroOrDefault(opts.merge.Name, opts.base.Name)
	out.Image = reflectutils.UseNonZeroOrDefault(opts.merge.Image, opts.base.Image)
	out.Command = structutils.MergeList(opts.base.Command, opts.merge.Command)
	out.Args = structutils.MergeList(opts.base.Args, opts.merge.Args)
	out.Env = structutils.MergeList(opts.base.Env, opts.merge.Env)
	out.WorkingDir = reflectutils.UseNonZeroOrDefault(opts.merge.WorkingDir, opts.base.WorkingDir)
	out.Ports = structutils.MergeList(opts.base.Ports, opts.merge.Ports)
	out.Resources = reflectutils.UseNonZeroOrDefault(opts.merge.Resources, opts.base.Resources)
	out.ResizePolicy = structutils.MergeList(opts.base.ResizePolicy, opts.merge.ResizePolicy)
	out.RestartPolicy = reflectutils.UseNonZeroOrDefault(opts.merge.RestartPolicy, opts.base.RestartPolicy)
	out.VolumeMounts = structutils.MergeList(opts.base.VolumeMounts, opts.merge.VolumeMounts)
	out.VolumeDevices = structutils.MergeList(opts.base.VolumeDevices, opts.merge.VolumeDevices)
	out.LivenessProbe = reflectutils.UseNonZeroOrDefault(opts.merge.LivenessProbe, opts.base.LivenessProbe)
	out.ReadinessProbe = reflectutils.UseNonZeroOrDefault(opts.merge.ReadinessProbe, opts.base.ReadinessProbe)
	out.StartupProbe = reflectutils.UseNonZeroOrDefault(opts.merge.StartupProbe, opts.base.StartupProbe)
	out.Lifecycle = reflectutils.UseNonZeroOrDefault(opts.merge.Lifecycle, opts.base.Lifecycle)
	out.TerminationMessagePath = reflectutils.UseNonZeroOrDefault(opts.merge.TerminationMessagePath, opts.base.TerminationMessagePath)
	out.TerminationMessagePolicy = reflectutils.UseNonZeroOrDefault(opts.merge.TerminationMessagePolicy, opts.base.TerminationMessagePolicy)
	out.ImagePullPolicy = reflectutils.UseNonZeroOrDefault(opts.merge.ImagePullPolicy, opts.base.ImagePullPolicy)
	out.SecurityContext = reflectutils.UseNonZeroOrDefault(opts.merge.SecurityContext, opts.base.SecurityContext)
	out.Stdin = reflectutils.UseNonZeroOrDefault(opts.merge.Stdin, opts.base.Stdin)
	out.StdinOnce = reflectutils.UseNonZeroOrDefault(opts.merge.StdinOnce, opts.base.StdinOnce)
	out.TTY = reflectutils.UseNonZeroOrDefault(opts.merge.TTY, opts.base.TTY)

	return out
}
