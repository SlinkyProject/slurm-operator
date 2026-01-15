// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/reflectutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
	"k8s.io/utils/ptr"
)

func (b *ControllerBuilder) BuildControllerServiceMonitor(controller *slinkyv1beta1.Controller) (*monitoringv1.ServiceMonitor, error) {
	serviceMonitor := controller.Spec.Metrics.ServiceMonitor

	opts := ServiceMonitorOpts{
		Key: controller.Key(),
		Metadata: slinkyv1beta1.Metadata{
			Annotations: structutils.MergeMaps(controller.Annotations, controller.Spec.Metrics.ServiceMonitor.Annotations),
			Labels:      structutils.MergeMaps(controller.Labels, controller.Spec.Metrics.ServiceMonitor.Labels, labels.NewBuilder().WithControllerLabels(controller).Build()),
		},
		base: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: labels.NewBuilder().WithControllerSelectorLabels(controller).Build(),
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{controller.Namespace},
			},
		},
	}

	metricEndpoints := controller.Spec.Metrics.ServiceMonitor.MetricEndpoints
	if len(metricEndpoints) > 0 {
		endpoints := make([]monitoringv1.Endpoint, 0, len(metricEndpoints))
		for _, metricEndpoint := range metricEndpoints {
			endpoint := monitoringv1.Endpoint{
				Path:          metricEndpoint.Path,
				Port:          labels.ControllerApp,
				Scheme:        ptr.To(monitoringv1.Scheme("http")),
				Interval:      reflectutils.UseNonZeroOrDefault(metricEndpoint.Interval, serviceMonitor.Interval),
				ScrapeTimeout: reflectutils.UseNonZeroOrDefault(metricEndpoint.ScrapeTimeout, serviceMonitor.ScrapeTimeout),
			}
			endpoints = append(endpoints, endpoint)
		}
		opts.base.Endpoints = append(opts.base.Endpoints, endpoints...)
	} else {
		newMetricsEndpoint := func(path string) monitoringv1.Endpoint {
			return monitoringv1.Endpoint{
				Path:          path,
				Port:          labels.ControllerApp,
				Scheme:        ptr.To(monitoringv1.Scheme("http")),
				Interval:      serviceMonitor.Interval,
				ScrapeTimeout: serviceMonitor.ScrapeTimeout,
			}
		}
		defaultEndpoints := []monitoringv1.Endpoint{
			newMetricsEndpoint("/metrics/jobs"),
			newMetricsEndpoint("/metrics/nodes"),
			newMetricsEndpoint("/metrics/partitions"),
			newMetricsEndpoint("/metrics/scheduler"),
		}
		opts.base.Endpoints = append(opts.base.Endpoints, defaultEndpoints...)
	}

	return b.BuildServiceMonitor(opts, controller)
}
