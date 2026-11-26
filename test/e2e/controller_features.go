// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"

	"github.com/SlinkyProject/slurm-operator/test/k8sutils"
	"github.com/SlinkyProject/slurm-operator/test/slurm"
)

func testSlurmControllerReconfigure(namespace string) types.Feature {
	feature := features.New("Assess Slurm controller configuration updates")
	for _, inplace := range []bool{false, true} {
		feature.Assess(fmt.Sprintf("configuration updates with inplaceReconfigure=%t", inplace), func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err)
			values := fmt.Sprintf("controller:\n  inplaceReconfigure: %t\n  extraConfMap:\n    SlurmctldDebug: info\nconfigFiles:\n  e2e.conf: before\n", inplace)
			upgradeSlurm(t, config, namespace, values)
			podKey := crclient.ObjectKey{Namespace: namespace, Name: "slurm-controller-0"}
			before := &corev1.Pod{}
			require.Eventually(t, func() bool {
				if err := crClient.Get(ctx, podKey, before); err != nil || !k8sutils.PodReady(before) || before.DeletionTimestamp != nil {
					return false
				}
				// Wait for the mode switch and its initial configuration to finish.
				for _, container := range before.Spec.InitContainers {
					if container.Name == "reconfigure" {
						return inplace && controllerConfigMatches(ctx, namespace, "info", "before")
					}
				}
				return !inplace && controllerConfigMatches(ctx, namespace, "info", "before")
			}, 3*time.Minute, 2*time.Second, "controller did not adopt initial configuration")

			upgradeSlurm(t, config, namespace, strings.ReplaceAll(strings.ReplaceAll(values, "info", "debug"), "before", "after"))
			after := &corev1.Pod{}
			require.EventuallyWithT(t, func(collect *assert.CollectT) {
				if !assert.NoError(collect, crClient.Get(ctx, podKey, after)) {
					return
				}
				assert.True(collect, k8sutils.PodReady(after) && after.DeletionTimestamp == nil)
				assert.True(collect, controllerConfigMatches(ctx, namespace, "debug", "after"), "effective Slurm configuration did not update")
				if inplace {
					assert.Equal(collect, before.UID, after.UID, "inplace reconfigure replaced the pod")
					assert.Equal(collect, before.Status.ContainerStatuses, after.Status.ContainerStatuses, "inplace reconfigure restarted a container")
					assert.Equal(collect, before.Status.InitContainerStatuses, after.Status.InitContainerStatuses, "inplace reconfigure restarted a sidecar")
				} else {
					assert.NotEqual(collect, before.UID, after.UID, "configuration update did not replace the pod")
				}
			}, 3*time.Minute, 2*time.Second, "controller configuration update failed")
			return ctx
		})
	}
	return feature.Feature()
}

func controllerConfigMatches(ctx context.Context, namespace, debug, contents string) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, "slurm-controller-0", "-c", "slurmctld", "--", //nolint:gosec // Test-only code
		"sh", "-ec", `scontrol show config | grep -E "^SlurmctldDebug[[:space:]]*=[[:space:]]*$1$"; test "$(cat /etc/slurm/e2e.conf)" = "$2"`, "config-check", debug, contents).CombinedOutput()
	return err == nil && len(output) > 0
}

func testSlurmControllerEphemeral(namespace string) types.Feature {
	return features.New("Assess Slurm controller without persistence").
		Assess("controller uses ephemeral state and can execute jobs", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err)
			pod := &corev1.Pod{}
			require.NoError(t, crClient.Get(ctx, crclient.ObjectKey{Namespace: namespace, Name: "slurm-controller-0"}, pod))
			found := false
			for _, volume := range pod.Spec.Volumes {
				if volume.Name == "statesave" {
					require.NotNil(t, volume.EmptyDir, "controller state must use emptyDir when persistence is disabled")
					found = true
				}
			}
			require.True(t, found, "controller has no state volume")
			waitForSrunHostname(ctx, t, slurm.NewClient(config.Client().RESTConfig(), namespace).InPod(pod.Name),
				"0:01", "slinky-0", time.Minute, 2*time.Second)
			return ctx
		}).Feature()
}
