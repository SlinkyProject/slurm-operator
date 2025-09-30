// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
	"github.com/SlinkyProject/slurm-operator/internal/utils/reflectutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

type PodTemplateOpts struct {
	Key      types.NamespacedName
	Metadata slinkyv1beta1.Metadata
	base     corev1.PodSpec
	merge    corev1.PodSpec
}

func (b *Builder) buildPodTemplate(opts PodTemplateOpts) corev1.PodTemplateSpec {
	objectMeta := metadata.NewBuilder(opts.Key).
		WithMetadata(opts.Metadata).
		Build()

	out := corev1.PodTemplateSpec{
		ObjectMeta: objectMeta,
		Spec:       opts.base,
	}

	out.Spec.Volumes = structutils.MergeList(out.Spec.Volumes, opts.merge.Volumes)
	out.Spec.InitContainers = structutils.MergeList(out.Spec.InitContainers, opts.merge.InitContainers)
	out.Spec.Containers = structutils.MergeList(out.Spec.Containers, opts.merge.Containers)
	out.Spec.EphemeralContainers = structutils.MergeList(out.Spec.EphemeralContainers, opts.merge.EphemeralContainers)
	out.Spec.RestartPolicy = reflectutils.UseNonZeroOrDefault(opts.merge.RestartPolicy, opts.base.RestartPolicy)
	out.Spec.TerminationGracePeriodSeconds = reflectutils.UseNonZeroOrDefault(opts.merge.TerminationGracePeriodSeconds, opts.base.TerminationGracePeriodSeconds)
	out.Spec.ActiveDeadlineSeconds = reflectutils.UseNonZeroOrDefault(opts.merge.ActiveDeadlineSeconds, opts.base.ActiveDeadlineSeconds)
	out.Spec.DNSPolicy = reflectutils.UseNonZeroOrDefault(opts.merge.DNSPolicy, opts.base.DNSPolicy)
	out.Spec.NodeSelector = structutils.MergeMaps(out.Spec.NodeSelector, opts.merge.NodeSelector)
	out.Spec.ServiceAccountName = reflectutils.UseNonZeroOrDefault(opts.merge.ServiceAccountName, opts.base.ServiceAccountName)
	out.Spec.DeprecatedServiceAccount = reflectutils.UseNonZeroOrDefault(opts.merge.DeprecatedServiceAccount, opts.base.DeprecatedServiceAccount)
	out.Spec.AutomountServiceAccountToken = reflectutils.UseNonZeroOrDefault(opts.merge.AutomountServiceAccountToken, opts.base.AutomountServiceAccountToken)
	out.Spec.NodeName = reflectutils.UseNonZeroOrDefault(opts.merge.NodeName, opts.base.NodeName)
	out.Spec.HostNetwork = reflectutils.UseNonZeroOrDefault(opts.merge.HostNetwork, opts.base.HostNetwork)
	out.Spec.HostPID = reflectutils.UseNonZeroOrDefault(opts.merge.HostPID, opts.base.HostPID)
	out.Spec.HostIPC = reflectutils.UseNonZeroOrDefault(opts.merge.HostIPC, opts.base.HostIPC)
	out.Spec.ShareProcessNamespace = reflectutils.UseNonZeroOrDefault(opts.merge.ShareProcessNamespace, opts.base.ShareProcessNamespace)
	out.Spec.SecurityContext = reflectutils.UseNonZeroOrDefault(opts.merge.SecurityContext, opts.base.SecurityContext)
	out.Spec.ImagePullSecrets = structutils.MergeList(out.Spec.ImagePullSecrets, opts.merge.ImagePullSecrets)
	out.Spec.Hostname = reflectutils.UseNonZeroOrDefault(opts.merge.Hostname, opts.base.Hostname)
	out.Spec.Subdomain = reflectutils.UseNonZeroOrDefault(opts.merge.Subdomain, opts.base.Subdomain)
	out.Spec.Affinity = reflectutils.UseNonZeroOrDefault(opts.merge.Affinity, opts.base.Affinity)
	out.Spec.Tolerations = structutils.MergeList(out.Spec.Tolerations, opts.merge.Tolerations)
	out.Spec.PriorityClassName = reflectutils.UseNonZeroOrDefault(opts.merge.PriorityClassName, opts.base.PriorityClassName)
	out.Spec.Priority = reflectutils.UseNonZeroOrDefault(opts.merge.Priority, opts.base.Priority)
	out.Spec.DNSConfig = reflectutils.UseNonZeroOrDefault(opts.merge.DNSConfig, opts.base.DNSConfig)
	out.Spec.ReadinessGates = structutils.MergeList(out.Spec.ReadinessGates, opts.merge.ReadinessGates)
	out.Spec.RuntimeClassName = reflectutils.UseNonZeroOrDefault(opts.merge.RuntimeClassName, opts.base.RuntimeClassName)
	out.Spec.EnableServiceLinks = reflectutils.UseNonZeroOrDefault(opts.merge.EnableServiceLinks, opts.base.EnableServiceLinks)
	out.Spec.PreemptionPolicy = reflectutils.UseNonZeroOrDefault(opts.merge.PreemptionPolicy, opts.base.PreemptionPolicy)
	out.Spec.Overhead = reflectutils.UseNonZeroOrDefault(opts.merge.Overhead, opts.base.Overhead)
	out.Spec.TopologySpreadConstraints = structutils.MergeList(out.Spec.TopologySpreadConstraints, opts.merge.TopologySpreadConstraints)
	out.Spec.SetHostnameAsFQDN = reflectutils.UseNonZeroOrDefault(opts.merge.SetHostnameAsFQDN, opts.base.SetHostnameAsFQDN)
	out.Spec.OS = reflectutils.UseNonZeroOrDefault(opts.merge.OS, opts.base.OS)
	out.Spec.HostUsers = reflectutils.UseNonZeroOrDefault(opts.merge.HostUsers, opts.base.HostUsers)
	out.Spec.SchedulingGates = structutils.MergeList(out.Spec.SchedulingGates, opts.merge.SchedulingGates)
	out.Spec.ResourceClaims = structutils.MergeList(out.Spec.ResourceClaims, opts.merge.ResourceClaims)
	out.Spec.Resources = reflectutils.UseNonZeroOrDefault(opts.merge.Resources, opts.base.Resources)
	out.Spec.HostnameOverride = reflectutils.UseNonZeroOrDefault(opts.merge.HostnameOverride, opts.base.HostnameOverride)

	return out
}
