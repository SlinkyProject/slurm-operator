// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
	"github.com/SlinkyProject/slurm-operator/internal/utils/reflectutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

type ServiceMonitorOpts struct {
	Key      types.NamespacedName
	Metadata slinkyv1beta1.Metadata
	base     monitoringv1.ServiceMonitorSpec
	merge    monitoringv1.ServiceMonitorSpec
}

func (b *ControllerBuilder) BuildServiceMonitor(opts ServiceMonitorOpts, owner metav1.Object) (*monitoringv1.ServiceMonitor, error) {
	objectMeta := metadata.NewBuilder(opts.Key).
		WithMetadata(opts.Metadata).
		Build()

	out := &monitoringv1.ServiceMonitor{
		ObjectMeta: objectMeta,
		Spec:       opts.base,
	}

	out.Spec.JobLabel = reflectutils.UseNonZeroOrDefault(opts.merge.JobLabel, opts.base.JobLabel)
	out.Spec.TargetLabels = structutils.MergeList(opts.base.TargetLabels, opts.merge.TargetLabels)
	out.Spec.PodTargetLabels = structutils.MergeList(opts.base.PodTargetLabels, opts.merge.PodTargetLabels)
	out.Spec.Endpoints = structutils.MergeList(opts.base.Endpoints, opts.merge.Endpoints)
	out.Spec.Selector = reflectutils.UseNonZeroOrDefault(opts.merge.Selector, opts.base.Selector)
	out.Spec.SelectorMechanism = reflectutils.UseNonZeroOrDefault(opts.merge.SelectorMechanism, opts.base.SelectorMechanism)
	out.Spec.NamespaceSelector = reflectutils.UseNonZeroOrDefault(opts.merge.NamespaceSelector, opts.base.NamespaceSelector)
	out.Spec.SampleLimit = reflectutils.UseNonZeroOrDefault(opts.merge.SampleLimit, opts.base.SampleLimit)
	out.Spec.ScrapeProtocols = structutils.MergeList(opts.base.ScrapeProtocols, opts.merge.ScrapeProtocols)
	out.Spec.FallbackScrapeProtocol = reflectutils.UseNonZeroOrDefault(opts.merge.FallbackScrapeProtocol, opts.base.FallbackScrapeProtocol)
	out.Spec.TargetLimit = reflectutils.UseNonZeroOrDefault(opts.merge.TargetLimit, opts.base.TargetLimit)
	out.Spec.LabelLimit = reflectutils.UseNonZeroOrDefault(opts.merge.LabelLimit, opts.base.LabelLimit)
	out.Spec.LabelNameLengthLimit = reflectutils.UseNonZeroOrDefault(opts.merge.LabelNameLengthLimit, opts.base.LabelNameLengthLimit)
	out.Spec.LabelValueLengthLimit = reflectutils.UseNonZeroOrDefault(opts.merge.LabelValueLengthLimit, opts.base.LabelValueLengthLimit)
	out.Spec.NativeHistogramConfig = reflectutils.UseNonZeroOrDefault(opts.merge.NativeHistogramConfig, opts.base.NativeHistogramConfig)
	out.Spec.KeepDroppedTargets = reflectutils.UseNonZeroOrDefault(opts.merge.KeepDroppedTargets, opts.base.KeepDroppedTargets)
	out.Spec.AttachMetadata = reflectutils.UseNonZeroOrDefault(opts.merge.AttachMetadata, opts.base.AttachMetadata)
	out.Spec.ScrapeClassName = reflectutils.UseNonZeroOrDefault(opts.merge.ScrapeClassName, opts.base.ScrapeClassName)
	out.Spec.BodySizeLimit = reflectutils.UseNonZeroOrDefault(opts.merge.BodySizeLimit, opts.base.BodySizeLimit)
	out.Spec.ServiceDiscoveryRole = reflectutils.UseNonZeroOrDefault(opts.merge.ServiceDiscoveryRole, opts.base.ServiceDiscoveryRole)

	if err := controllerutil.SetControllerReference(owner, out, b.client.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to set owner controller: %w", err)
	}

	return out, nil
}
