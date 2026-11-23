// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"testing"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"
)

func testSlurmMetrics(namespace string) types.Feature {
	return features.New("Assess Slurm controller metrics").
		Assess("metrics endpoints are scraped and monitoring can be disabled", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err)
			require.NoError(t, monitoringv1.AddToScheme(crClient.Scheme()))
			monitorKey := crclient.ObjectKey{Namespace: namespace, Name: "slurm-controller"}
			monitor := &monitoringv1.ServiceMonitor{}
			require.Eventually(t, func() bool {
				return crClient.Get(ctx, monitorKey, monitor) == nil
			}, time.Minute, time.Second, "controller ServiceMonitor was not created")
			require.Len(t, monitor.Spec.Endpoints, 4)

			get := func(path string) ([]byte, error) {
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				return exec.CommandContext(ctx, "kubectl", "get", "--raw", path).CombinedOutput() //nolint:gosec // Test-only code
			}
			for _, endpoint := range monitor.Spec.Endpoints {
				path := fmt.Sprintf("/api/v1/namespaces/%s/services/http:slurm-controller:slurmctld/proxy%s", namespace, endpoint.Path)
				require.EventuallyWithT(t, func(collect *assert.CollectT) {
					output, err := get(path)
					assert.NoError(collect, err, "metrics request failed: %s", output)
					assert.Contains(collect, string(output), "# HELP", "endpoint %s did not return metric metadata", endpoint.Path)
					assert.Contains(collect, string(output), "# TYPE", "endpoint %s did not return metric types", endpoint.Path)
				}, time.Minute, 2*time.Second, "metrics endpoint %s is unavailable", endpoint.Path)
			}

			path := "/api/v1/namespaces/monitoring/services/http:prometheus-kube-prometheus-prometheus:9090/proxy/api/v1/targets?state=active"
			require.EventuallyWithT(t, func(collect *assert.CollectT) {
				pod := &corev1.Pod{}
				if !assert.NoError(collect, crClient.Get(ctx, crclient.ObjectKey{Namespace: namespace, Name: "slurm-controller-0"}, pod)) {
					return
				}
				output, err := get(path)
				if !assert.NoError(collect, err, "Prometheus targets request failed: %s", output) {
					return
				}
				var response struct {
					Status string `json:"status"`
					Data   struct {
						ActiveTargets []struct {
							Labels    map[string]string `json:"labels"`
							ScrapeURL string            `json:"scrapeUrl"`
							Health    string            `json:"health"`
						} `json:"activeTargets"`
					} `json:"data"`
				}
				if !assert.NoError(collect, json.Unmarshal(output, &response)) {
					return
				}
				assert.Equal(collect, "success", response.Status)
				healthy := map[string]bool{}
				for _, target := range response.Data.ActiveTargets {
					if target.Labels["namespace"] != namespace || target.Labels["service"] != "slurm-controller" ||
						target.Labels["instance"] != net.JoinHostPort(pod.Status.PodIP, "6817") {
						continue
					}
					endpoint, err := url.Parse(target.ScrapeURL)
					if assert.NoError(collect, err) {
						healthy[endpoint.Path] = target.Health == "up"
					}
				}
				for _, endpoint := range monitor.Spec.Endpoints {
					assert.True(collect, healthy[endpoint.Path], "metrics target %s was not discovered and scraped successfully", endpoint.Path)
				}
			}, 3*time.Minute, 5*time.Second, "Prometheus did not successfully scrape controller metrics")

			upgradeSlurm(t, config, namespace, "controller:\n  metrics:\n    enabled: false\n    serviceMonitor:\n      enabled: false\n")
			require.Eventually(t, func() bool {
				return apierrors.IsNotFound(crClient.Get(ctx, monitorKey, &monitoringv1.ServiceMonitor{}))
			}, time.Minute, time.Second, "disabled controller ServiceMonitor was not removed")
			return ctx
		}).Feature()
}
