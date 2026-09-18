// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"

	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	clienttoken "github.com/SlinkyProject/slurm-client/pkg/client/token"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/token/slurmjwt"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/SlinkyProject/slurm-operator/test"
)

// Dependency Component Health Checks

func checkMariaDBHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, namespace string) context.Context {
	t.Helper()

	// Get MariaDB CR

	mariadb := &mariadbv1alpha1.MariaDB{}

	mariadbKey := crclient.ObjectKey{
		Namespace: namespace,
		Name:      "mariadb",
	}

	err := crClient.Get(ctx, mariadbKey, mariadb)
	require.NoError(t, err, "failed to Get() mariadb using controller-runtime client")

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "mariadb",
		},
	}
	err = wait.For(
		conditions.New(config.Client().Resources()).ResourceScaled(statefulSet, func(object k8s.Object) int32 {
			return object.(*appsv1.StatefulSet).Status.ReadyReplicas
		}, 1),
		wait.WithContext(ctx),
		wait.WithTimeout(10*time.Minute),
		wait.WithInterval(5*time.Second),
		wait.WithImmediate(),
	)
	require.NoError(
		t,
		err,
		"timed out waiting for StatefulSet %s/%s to reach one ready replica; observed status: %s",
		statefulSet.Namespace,
		statefulSet.Name,
		test.StatusJSON(statefulSet.Status),
	)

	return ctx
}

// Slinky Component Health Checks

// Controller tests

func checkControllerHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, namespace string) {
	t.Helper()

	// Get Controller CR
	controller := &slinkyv1beta1.Controller{}

	controllerKey := crclient.ObjectKey{
		Namespace: namespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, controllerKey, controller)
	require.NoError(t, err, "failed to Get() controller using controller-runtime client")

	controllerUID := controller.UID

	// Get Controller StatefulSet using controller CR
	statefulSetKey := controller.Key()
	statefulSet := &appsv1.StatefulSet{}
	err = crClient.Get(ctx, statefulSetKey, statefulSet)
	require.NoError(t, err, "failed to Get() statefulset using controller-runtime client")

	// Confirm ownership of controller statefulset
	for _, owner := range statefulSet.OwnerReferences {
		require.Equal(t, controllerUID, owner.UID, "dubious ownership of statefulset: %v", statefulSet)
	}

	// Wait for controller statefulset to become ready
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(statefulSet, func(object k8s.Object) int32 {
		return object.(*appsv1.StatefulSet).Status.ReadyReplicas
	}, *statefulSet.Spec.Replicas))
	if err != nil {
		_ = crClient.Get(ctx, controllerKey, controller)
		t.Fatalf(
			"timed out waiting for controller StatefulSet %s/%s to reach %d ready replicas: %v; controller status: %s; StatefulSet status: %s",
			statefulSet.Namespace,
			statefulSet.Name,
			*statefulSet.Spec.Replicas,
			err,
			test.StatusJSON(controller.Status),
			test.StatusJSON(statefulSet.Status),
		)
	}
}

func testSlurmController(namespace string) types.Feature {
	return features.New("Assess the functionality of the Slurm controller").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("slurmctld is responsive", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "scontrol", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("slurm controller can resolve nodeset by hostname", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			checkHostnameResolution(ctx, t, namespace, "slurm-controller-0", "slinky-0")

			return ctx
		}).
		Assess("job launch & execution succeeds (srun)", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "srun", "--immediate=10", "-K", "-Q", "--time=0:15", "hostname"}
			wants := "slinky-0"

			cleanup_command := "kubectl"
			cleanup_args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"}

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).Feature()
}

func checkHostnameResolution(ctx context.Context, t *testing.T, namespace, sourcePod, nodeName string) {
	t.Helper()

	const attempts = 16
	var (
		lastAddress string
		lastErr     error
		lastOutput  []byte
	)

	for attempt := range attempts {
		nodeInfo, err := test.GetSlurmNodeInfo(ctx, namespace, nodeName)
		if err != nil {
			lastErr = err
		} else {
			lastAddress = nodeInfo["NodeAddr"]
			if lastAddress == "" {
				lastErr = nil
				lastOutput = nil
			} else {
				args := []string{
					"exec", "-n", namespace, sourcePod, "--",
					"getent", "hosts", lastAddress,
				}
				cmd := exec.CommandContext(ctx, "kubectl", args...)
				lastOutput, lastErr = cmd.CombinedOutput()
				fields := strings.Fields(string(lastOutput))
				if lastErr == nil && len(fields) > 1 && strings.HasPrefix(fields[len(fields)-1], nodeName) {
					return
				}
			}
		}

		if attempt < attempts-1 {
			select {
			case <-ctx.Done():
				t.Fatalf(
					"context ended while resolving Slurm node %q from pod %q: address=%q, last command error=%v, last combined output=%q: %v",
					nodeName,
					sourcePod,
					lastAddress,
					lastErr,
					strings.TrimSpace(string(lastOutput)),
					ctx.Err(),
				)
			case <-time.After(5 * time.Second):
			}
		}
	}

	t.Fatalf(
		"failed to resolve Slurm node %q from pod %q after %d attempts: address=%q, last command error=%v, last combined output=%q",
		nodeName,
		sourcePod,
		attempts,
		lastAddress,
		lastErr,
		strings.TrimSpace(string(lastOutput)),
	)
}

// RestAPI tests

func checkRestAPIHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, namespace string) {
	t.Helper()

	// Get RestAPI CR
	restapi := &slinkyv1beta1.RestApi{}

	restapiKey := crclient.ObjectKey{
		Namespace: namespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, restapiKey, restapi)
	require.NoError(t, err, "failed to Get() restapi using controller-runtime client")

	restapiUID := restapi.UID

	// Get RestAPI Deployment using RestAPI CR
	deploymentKey := restapi.Key()
	deployment := &appsv1.Deployment{}
	err = crClient.Get(ctx, deploymentKey, deployment)
	require.NoError(t, err, "failed to Get() deployment using controller-runtime client")

	// Confirm ownership of RestAPI deployment
	for _, owner := range deployment.OwnerReferences {
		require.Equal(t, restapiUID, owner.UID, "dubious ownership of deployment: %v", deployment)
	}

	// Check whether RestAPI deployment is healthy
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, *deployment.Spec.Replicas))
	if err != nil {
		_ = crClient.Get(ctx, restapiKey, restapi)
		t.Fatalf(
			"timed out waiting for REST API Deployment %s/%s to reach %d ready replicas: %v; REST API status: %s; Deployment status: %s",
			deployment.Namespace,
			deployment.Name,
			*deployment.Spec.Replicas,
			err,
			test.StatusJSON(restapi.Status),
			test.StatusJSON(deployment.Status),
		)
	}
}

func testSlurmRestAPI(namespace string, withAccounting bool) types.Feature {
	return features.New("Assess the functionality of the Slurm RestAPI").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("slurmrestd container args match expectations", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"get", "deployment", "-n", namespace, "slurm-restapi", "-o", `jsonpath="{.spec.template.spec.containers[0].args}"`}
			var wants string
			if withAccounting {
				wants = `"["0.0.0.0:6820"]"`
			} else {
				wants = `"["-s","openapi/slurmctld","0.0.0.0:6820"]"`
			}

			cmd := exec.Command(command, args...)
			output, err := cmd.Output()

			if strings.TrimSpace(string(output)) != wants {
				require.NoError(t, err, "failed running %v %v", command, args)
				require.Equal(t, wants, strings.TrimSpace(string(output)))
			}

			return ctx
		}).Feature()
}

// NodeSet tests

const (
	// topologySyncDisabledObservation spans the NodeSet controller's 30-second periodic reconcile.
	topologySyncDisabledObservation = 35 * time.Second
	topologyReadTimeout             = 5 * time.Second
)

func checkNodeSetReplicas(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, nodesetKey crclient.ObjectKey) {
	t.Helper()

	nodeset := &slinkyv1beta1.NodeSet{}
	started := time.Now()

	for retry := range 16 {

		err := crClient.Get(ctx, nodesetKey, nodeset)
		require.NoError(t, err, "failed to Get() NodeSet using controller-runtime client")

		if *nodeset.Spec.Replicas == nodeset.Status.AvailableReplicas {
			break
		}

		if retry == 15 {
			t.Fatalf(
				"timed out after %s waiting for NodeSet %s/%s replicas to become available: spec.replicas=%d; observed status=%s",
				time.Since(started).Round(time.Millisecond),
				nodeset.Namespace,
				nodeset.Name,
				*nodeset.Spec.Replicas,
				test.StatusJSON(nodeset.Status),
			)
		}

		time.Sleep(5 * time.Second)
	}
}

func testSlurmNodeSet(namespace string) types.Feature {
	return features.New("Assess the functionality of the Slurm NodeSet").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Nodeset can contact controller", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-worker-slinky-0", "--", "scontrol", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 4*time.Minute, 5*time.Second)

			return ctx
		}).
		Assess("NodeSet is idle", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-worker-slinky-0", "--", "sinfo", "-N", "-n", "slinky-0", "--Format=StateLong", "-h"}
			wants := "idle"

			cleanup_command := "kubectl"
			cleanup_args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"}

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("NodeSet scale-up functions", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "Failed to get new controller-runtime client")

			nodesetKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky",
			}
			nodeset := &slinkyv1beta1.NodeSet{}
			err = crClient.Get(ctx, nodesetKey, nodeset)
			require.NoError(t, err, "failed to Get() NodeSet using controller-runtime client")

			var replicas int32 = 2
			nodeset.Spec.Replicas = &replicas

			err = crClient.Update(ctx, nodeset)
			require.NoError(t, err, "failed to Update() NodeSet using controller-runtime client")

			checkNodeSetReplicas(crClient, ctx, t, config, nodesetKey)

			return ctx
		}).
		Assess("NodeSets can resolve each other's hostnames", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			checkHostnameResolution(ctx, t, namespace, "slurm-worker-slinky-0", "slinky-1")

			return ctx
		}).
		Assess("NodeSet scale-down functions", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "Failed to get new controller-runtime client")

			nodesetKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky",
			}
			nodeset := &slinkyv1beta1.NodeSet{}
			err = crClient.Get(ctx, nodesetKey, nodeset)
			require.NoError(t, err, "failed to Get() NodeSet using controller-runtime client")

			var replicas int32 = 1
			nodeset.Spec.Replicas = &replicas

			err = crClient.Update(ctx, nodeset)
			require.NoError(t, err, "failed to Update() NodeSet using controller-runtime client")

			checkNodeSetReplicas(crClient, ctx, t, config, nodesetKey)

			return ctx
		}).Feature()
}

func testSlurmNodeSetTopologySync(namespace string) types.Feature {
	return features.New("Assess NodeSet topology annotation synchronization").
		Assess("syncTopology controls topology annotation synchronization", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			nodesetKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky",
			}
			nodeset := &slinkyv1beta1.NodeSet{}
			require.NoError(t, crClient.Get(ctx, nodesetKey, nodeset), "failed to get NodeSet")

			podKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky-0",
			}
			workerPod := &corev1.Pod{}
			require.NoError(t, crClient.Get(ctx, podKey, workerPod), "failed to get NodeSet Pod")
			require.NotEmpty(t, workerPod.Spec.NodeName, "NodeSet Pod %s/%s has empty spec.nodeName", workerPod.Namespace, workerPod.Name)

			slurmNodeName := workerPod.Labels[slinkyv1beta1.LabelNodeSetPodHostname]
			require.NotEmpty(t, slurmNodeName, "NodeSet Pod %s/%s has no Slurm node name label", workerPod.Namespace, workerPod.Name)

			node := &corev1.Node{}
			nodeKey := crclient.ObjectKey{Name: workerPod.Spec.NodeName}
			require.NoError(t, crClient.Get(ctx, nodeKey, node), "failed to get Kubernetes Node %s", nodeKey.Name)

			var originalSyncTopology *bool
			if nodeset.Spec.SyncTopology != nil {
				originalSyncTopology = ptr.To(*nodeset.Spec.SyncTopology)
			}
			var originalNodeTopology *string
			if topology, ok := node.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec]; ok {
				originalNodeTopology = ptr.To(topology)
			}

			targetTopology := "topo-e2e:b0"
			if ptr.Deref(originalNodeTopology, "") == targetTopology {
				targetTopology = "topo-e2e:b1"
			}

			restored := false
			restore := func(restoreCtx context.Context) error {
				if err := updateNodeTopologyAnnotation(restoreCtx, crClient, nodeKey, originalNodeTopology); err != nil {
					return fmt.Errorf("restore Kubernetes Node topology annotation: %w", err)
				}
				if err := updateNodeSetSyncTopology(restoreCtx, crClient, nodesetKey, originalSyncTopology); err != nil {
					return fmt.Errorf("restore NodeSet topology synchronization: %w", err)
				}
				return nil
			}
			t.Cleanup(func() {
				if restored {
					return
				}
				cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				if err := restore(cleanupCtx); err != nil {
					t.Errorf("failed to clean up topology synchronization test: %v", err)
				}
			})

			waitForNodeSetTopology(ctx, t, crClient, podKey, namespace, slurmNodeName, ptr.Deref(originalNodeTopology, ""))

			require.NoError(
				t,
				updateNodeSetSyncTopology(ctx, crClient, nodesetKey, ptr.To(false)),
				"failed to disable NodeSet topology synchronization",
			)
			require.Eventually(t, func() bool {
				current := &slinkyv1beta1.NodeSet{}
				if err := crClient.Get(ctx, nodesetKey, current); err != nil {
					return false
				}
				return current.Spec.SyncTopology != nil &&
					!*current.Spec.SyncTopology &&
					current.Status.ObservedGeneration == current.Generation
			}, time.Minute, time.Second, "timed out waiting for NodeSet to observe syncTopology=false")

			originalPodTopology, originalSlurmTopology, err := readNodeSetTopologies(ctx, crClient, podKey, namespace, slurmNodeName)
			require.NoError(t, err, "failed to read topology before changing the Kubernetes Node annotation")

			require.NoError(
				t,
				updateNodeTopologyAnnotation(ctx, crClient, nodeKey, ptr.To(targetTopology)),
				"failed to update Kubernetes Node topology annotation",
			)
			checkNodeSetTopologyUnchanged(
				ctx,
				t,
				crClient,
				podKey,
				namespace,
				slurmNodeName,
				originalPodTopology,
				originalSlurmTopology,
			)

			require.NoError(
				t,
				updateNodeSetSyncTopology(ctx, crClient, nodesetKey, ptr.To(true)),
				"failed to enable NodeSet topology synchronization",
			)
			waitForNodeSetTopology(ctx, t, crClient, podKey, namespace, slurmNodeName, targetTopology)

			require.NoError(t, restore(ctx), "failed to restore topology synchronization test resources")
			restored = true

			return ctx
		}).Feature()
}

func nodeSetPods(ctx context.Context, crClient crclient.Client, nodeset *slinkyv1beta1.NodeSet) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := crClient.List(ctx, podList, crclient.InNamespace(nodeset.Namespace)); err != nil {
		return nil, err
	}

	pods := make([]corev1.Pod, 0, len(podList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]
		if metav1.IsControlledBy(pod, nodeset) {
			pods = append(pods, *pod)
		}
	}
	return pods, nil
}

// waitForDaemonSetReplicas waits for a DaemonSet-mode NodeSet and all of its
// pods to converge. Pass a negative expected value to accept the controller's
// non-zero desired count, which is useful during initial installation.
func waitForDaemonSetReplicas(
	crClient crclient.Client,
	ctx context.Context,
	t *testing.T,
	nodesetKey crclient.ObjectKey,
	expected int32,
) (*slinkyv1beta1.NodeSet, []corev1.Pod) {
	t.Helper()

	var (
		nodeset slinkyv1beta1.NodeSet
		pods    []corev1.Pod
	)

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		current := &slinkyv1beta1.NodeSet{}
		if !assert.NoError(
			collect,
			crClient.Get(ctx, nodesetKey, current),
			"failed to get NodeSet",
		) {
			return
		}

		currentPods, err := nodeSetPods(ctx, crClient, current)
		if !assert.NoError(collect, err, "failed to list NodeSet pods") {
			return
		}

		nodeset = *current
		pods = currentPods

		desired := expected
		if desired < 0 {
			desired = current.Status.Desired
			assert.NotZero(collect, desired, "desired replica count is not populated")
		}

		assert.Equal(collect, current.Generation, current.Status.ObservedGeneration)
		assert.Equal(collect, desired, current.Status.Desired)
		assert.Equal(collect, desired, current.Status.Replicas)
		assert.Equal(collect, desired, current.Status.UpdatedReplicas)
		assert.Equal(collect, desired, current.Status.ReadyReplicas)
		assert.Equal(collect, desired, current.Status.AvailableReplicas)
		assert.Len(collect, currentPods, int(desired))

		for i := range currentPods {
			assert.True(collect, podReady(&currentPods[i]), "pod %s is not ready", currentPods[i].Name)
		}
	}, 2*time.Minute, 2*time.Second,
		"DaemonSet-mode NodeSet %s/%s did not converge",
		nodesetKey.Namespace,
		nodesetKey.Name,
	)

	return &nodeset, pods
}

func testSlurmDaemonSet(namespace string) types.Feature {
	nodesetKey := crclient.ObjectKey{Namespace: namespace, Name: "slurm-worker-slinky"}
	var (
		initialDesired       int32
		originalNodeSelector map[string]string
		selectedNode         string
		workerHostname       string
		workerPod            string
	)

	return features.New("Assess DaemonSet scaling of the Slurm NodeSet").
		Assess("DaemonSet creates one ready Slurm worker per eligible Kubernetes node", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			nodeset, pods := waitForDaemonSetReplicas(crClient, ctx, t, nodesetKey, -1)
			require.Equal(t, slinkyv1beta1.ScalingModeDaemonset, nodeset.Spec.ScalingMode)
			require.Greater(t, nodeset.Status.Desired, int32(1), "DaemonSet scaling e2e test requires at least two eligible Kubernetes nodes")
			if nodeset.Spec.Replicas != nil {
				require.NotEqual(t, *nodeset.Spec.Replicas, nodeset.Status.Desired, "DaemonSet desired count must be node-driven, not spec.replicas")
			}

			initialDesired = nodeset.Status.Desired
			originalNodeSelector = make(map[string]string, len(nodeset.Spec.Template.PodSpecWrapper.NodeSelector))
			for key, value := range nodeset.Spec.Template.PodSpecWrapper.NodeSelector {
				originalNodeSelector[key] = value
			}

			seenNodes := make(map[string]struct{}, len(pods))
			for i := range pods {
				pod := &pods[i]
				require.NotEmpty(t, pod.Spec.NodeName, "DaemonSet pod %s was not assigned to a Kubernetes node", pod.Name)
				require.NotEmpty(t, pod.Spec.Hostname, "DaemonSet pod %s has no Slurm hostname", pod.Name)
				require.NotContains(t, seenNodes, pod.Spec.NodeName, "multiple DaemonSet pods were assigned to Kubernetes node %s", pod.Spec.NodeName)
				seenNodes[pod.Spec.NodeName] = struct{}{}
				require.Equal(t, string(slinkyv1beta1.ScalingModeDaemonset), pod.Labels[slinkyv1beta1.LabelNodeSetScalingMode])
				require.Empty(t, pod.Labels[slinkyv1beta1.LabelNodeSetPodIndex], "DaemonSet pod must not have a StatefulSet ordinal")
				require.Equal(t, pod.Spec.Hostname, pod.Labels[slinkyv1beta1.LabelNodeSetPodHostname])
			}

			selectedNode = pods[0].Spec.NodeName
			workerHostname = pods[0].Spec.Hostname
			workerPod = pods[0].Name
			return ctx
		}).
		Assess("DaemonSet worker participates in the Slurm cluster", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			test.WaitForCommand(
				ctx,
				t,
				"kubectl",
				[]string{"exec", "-n", namespace, workerPod, "--", "scontrol", "ping"},
				"",
				"",
				nil,
				80*time.Second,
				5*time.Second,
			)

			test.WaitForCommand(
				ctx,
				t,
				"kubectl",
				[]string{"exec", "-n", namespace, workerPod, "--", "sinfo", "-N", "-n", workerHostname, "--Format=StateLong", "-h"},
				"idle",
				"kubectl",
				[]string{"exec", "-n", namespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"},
				80*time.Second,
				5*time.Second,
			)

			checkHostnameResolution(ctx, t, namespace, "slurm-controller-0", workerHostname)
			return ctx
		}).
		Assess("DaemonSet scales down when the NodeSet selects one Kubernetes node", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			selectedKubeNode := &corev1.Node{}
			require.NoError(t, crClient.Get(ctx, crclient.ObjectKey{Name: selectedNode}, selectedKubeNode), "failed to get selected Kubernetes node")
			hostnameLabel := selectedKubeNode.Labels[corev1.LabelHostname]
			require.NotEmpty(t, hostnameLabel, "selected Kubernetes node %s has no %s label", selectedNode, corev1.LabelHostname)

			nodeset := &slinkyv1beta1.NodeSet{}
			require.NoError(t, crClient.Get(ctx, nodesetKey, nodeset), "failed to get NodeSet")
			nodeSelector := make(map[string]string, len(originalNodeSelector)+1)
			for key, value := range originalNodeSelector {
				nodeSelector[key] = value
			}
			nodeSelector[corev1.LabelHostname] = hostnameLabel
			nodeset.Spec.Template.PodSpecWrapper.NodeSelector = nodeSelector
			require.NoError(t, crClient.Update(ctx, nodeset), "failed to restrict DaemonSet NodeSet to one Kubernetes node")

			_, pods := waitForDaemonSetReplicas(crClient, ctx, t, nodesetKey, 1)
			require.Equal(t, selectedNode, pods[0].Spec.NodeName)
			return ctx
		}).
		Assess("DaemonSet scales up when the original node selector is restored", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			nodeset := &slinkyv1beta1.NodeSet{}
			require.NoError(t, crClient.Get(ctx, nodesetKey, nodeset), "failed to get NodeSet")
			nodeset.Spec.Template.PodSpecWrapper.NodeSelector = originalNodeSelector
			require.NoError(t, crClient.Update(ctx, nodeset), "failed to restore DaemonSet NodeSet node selector")

			_, pods := waitForDaemonSetReplicas(crClient, ctx, t, nodesetKey, initialDesired)
			seenNodes := make(map[string]struct{}, len(pods))
			for i := range pods {
				require.NotContains(t, seenNodes, pods[i].Spec.NodeName, "multiple DaemonSet pods were assigned to Kubernetes node %s", pods[i].Spec.NodeName)
				seenNodes[pods[i].Spec.NodeName] = struct{}{}
			}
			return ctx
		}).Feature()
}

func updateNodeSetSyncTopology(
	ctx context.Context,
	crClient crclient.Client,
	nodesetKey crclient.ObjectKey,
	enabled *bool,
) error {
	nodeset := &slinkyv1beta1.NodeSet{}
	if err := crClient.Get(ctx, nodesetKey, nodeset); err != nil {
		return fmt.Errorf("get NodeSet %s: %w", nodesetKey, err)
	}
	return objectutils.PatchObject(crClient, ctx, nodeset, func(nodeset *slinkyv1beta1.NodeSet) error {
		nodeset.Spec.SyncTopology = enabled
		return nil
	})
}

func updateNodeTopologyAnnotation(
	ctx context.Context,
	crClient crclient.Client,
	nodeKey crclient.ObjectKey,
	topology *string,
) error {
	node := &corev1.Node{}
	if err := crClient.Get(ctx, nodeKey, node); err != nil {
		return fmt.Errorf("get Kubernetes Node %s: %w", nodeKey, err)
	}
	return objectutils.PatchObject(crClient, ctx, node, func(node *corev1.Node) error {
		if topology == nil {
			delete(node.Annotations, slinkyv1beta1.AnnotationNodeTopologySpec)
			return nil
		}
		if node.Annotations == nil {
			node.Annotations = make(map[string]string)
		}
		node.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec] = *topology
		return nil
	})
}

func checkNodeSetTopologyUnchanged(
	ctx context.Context,
	t *testing.T,
	crClient crclient.Client,
	podKey crclient.ObjectKey,
	namespace string,
	slurmNodeName string,
	wantPodTopology string,
	wantSlurmTopology string,
) {
	t.Helper()

	deadline := time.Now().Add(topologySyncDisabledObservation)
	for {
		readCtx, cancel := context.WithTimeout(ctx, topologyReadTimeout)
		podTopology, slurmTopology, err := readNodeSetTopologies(readCtx, crClient, podKey, namespace, slurmNodeName)
		cancel()

		if ctx.Err() != nil {
			t.Fatalf("context ended while checking disabled topology synchronization: %v", ctx.Err())
		}
		require.NoError(t, err, "failed to read topology while syncTopology=false")
		require.Equal(t, wantPodTopology, podTopology, "NodeSet Pod topology changed while syncTopology=false")
		require.Equal(t, wantSlurmTopology, slurmTopology, "Slurm node topology changed while syncTopology=false")

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context ended while checking disabled topology synchronization: %v", ctx.Err())
		case <-time.After(min(time.Second, remaining)):
		}
	}
}

func readNodeSetTopologies(
	ctx context.Context,
	crClient crclient.Client,
	podKey crclient.ObjectKey,
	namespace string,
	slurmNodeName string,
) (string, string, error) {
	pod := &corev1.Pod{}
	if err := crClient.Get(ctx, podKey, pod); err != nil {
		return "", "", fmt.Errorf("get NodeSet Pod %s: %w", podKey, err)
	}
	nodeInfo, err := test.GetSlurmNodeInfo(ctx, namespace, slurmNodeName)
	if err != nil {
		return "", "", fmt.Errorf("get Slurm node %s: %w", slurmNodeName, err)
	}
	return pod.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec], nodeInfo["Topology"], nil
}

func waitForNodeSetTopology(
	ctx context.Context,
	t *testing.T,
	crClient crclient.Client,
	podKey crclient.ObjectKey,
	namespace string,
	slurmNodeName string,
	want string,
) {
	t.Helper()

	var (
		podTopology   string
		slurmTopology string
		lastErr       error
	)
	err := wait.For(
		func(waitCtx context.Context) (bool, error) {
			podTopology, slurmTopology, lastErr = readNodeSetTopologies(waitCtx, crClient, podKey, namespace, slurmNodeName)
			return lastErr == nil && podTopology == want && slurmTopology == want, nil
		},
		wait.WithContext(ctx),
		wait.WithTimeout(2*time.Minute),
		wait.WithInterval(2*time.Second),
		wait.WithImmediate(),
	)
	require.NoError(
		t,
		err,
		"timed out waiting for NodeSet topology synchronization: Pod got %q, Slurm got %q, want %q; last read error: %v",
		podTopology,
		slurmTopology,
		want,
		lastErr,
	)
}

func testSlurmJWTKeyRotation(namespace string) types.Feature {
	return features.New("Assess Slurm JWT signing key rotation").
		Assess("referenced JWT Secret updates refresh the Slurm client", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			controller := &slinkyv1beta1.Controller{}
			controllerKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm",
			}
			require.NoError(t, crClient.Get(ctx, controllerKey, controller), "failed to get Controller")

			jwtKeyRef := controller.AuthJwtRef()
			secretKey := crclient.ObjectKey{
				Namespace: controller.Namespace,
				Name:      jwtKeyRef.Name,
			}
			jwtSecret := &corev1.Secret{}
			require.NoError(t, crClient.Get(ctx, secretKey, jwtSecret), "failed to get JWT Secret")

			// The chart-generated Secret is immutable. Recreate it once with the
			// same data so the test can exercise a real Secret Update event.
			if jwtSecret.Immutable != nil && *jwtSecret.Immutable {
				mutableSecret := jwtSecret.DeepCopy()
				mutableSecret.ResourceVersion = ""
				mutableSecret.UID = ""
				mutableSecret.CreationTimestamp = metav1.Time{}
				mutableSecret.DeletionTimestamp = nil
				mutableSecret.DeletionGracePeriodSeconds = nil
				mutableSecret.ManagedFields = nil
				mutableSecret.Immutable = nil

				require.NoError(t, crClient.Delete(ctx, jwtSecret), "failed to delete immutable JWT Secret")
				require.Eventually(t, func() bool {
					current := &corev1.Secret{}
					return apierrors.IsNotFound(crClient.Get(ctx, secretKey, current))
				}, 30*time.Second, 500*time.Millisecond, "timed out waiting for immutable JWT Secret deletion")
				require.NoError(t, crClient.Create(ctx, mutableSecret), "failed to recreate mutable JWT Secret")
			}

			// Capture the current slurmctld pod. Updating the key should replace it
			// so Slurm and the operator both begin using the new signing key.
			controllerPodKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-controller-0",
			}
			oldControllerPod := &corev1.Pod{}
			require.NoError(t, crClient.Get(ctx, controllerPodKey, oldControllerPod), "failed to get slurmctld pod")

			randomKey := make([]byte, 64)
			_, err = rand.Read(randomKey)
			require.NoError(t, err, "failed to generate rotated JWT signing key")

			jwtSecret = &corev1.Secret{}
			require.NoError(t, crClient.Get(ctx, secretKey, jwtSecret), "failed to get mutable JWT Secret")
			jwtSecret.Data[jwtKeyRef.Key] = []byte(hex.EncodeToString(randomKey))
			require.NoError(t, crClient.Update(ctx, jwtSecret), "failed to update JWT Secret")

			require.Eventually(t, func() bool {
				current := &corev1.Pod{}
				if err := crClient.Get(ctx, controllerPodKey, current); err != nil {
					return false
				}
				return current.UID != oldControllerPod.UID && podReady(current)
			}, 2*time.Minute, 2*time.Second, "timed out waiting for slurmctld to adopt the rotated JWT key")

			authToken, err := slurmjwt.NewToken(jwtSecret.Data[jwtKeyRef.Key]).NewSignedToken()
			require.NoError(t, err, "failed to generate an auth token from the rotated JWT key")

			restConfig := rest.CopyConfig(config.Client().RESTConfig())
			restConfig.Timeout = 10 * time.Second
			httpClient, err := rest.HTTPClientFor(restConfig)
			require.NoError(t, err, "failed to create Kubernetes API HTTP client")

			server := fmt.Sprintf(
				"%s/api/v1/namespaces/%s/services/http:slurm-restapi:slurmrestd/proxy",
				strings.TrimRight(restConfig.Host, "/"),
				namespace,
			)
			operatorClient, err := slurmclient.NewClient(&slurmclient.Config{
				Server:        server,
				TokenProvider: clienttoken.StaticProvider(authToken),
				HTTPClient:    httpClient,
			})
			require.NoError(t, err, "failed to create Slurm client")

			require.Eventually(t, func() bool {
				pings := &slurmtypes.V0044ControllerPingList{}
				err := operatorClient.List(ctx, pings, &slurmclient.ListOptions{SkipCache: true})
				return err == nil
			}, 2*time.Minute, 2*time.Second, "timed out waiting for a controller ping using the rotated JWT key")

			return ctx
		}).Feature()
}

func podReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// Accounting tests

func checkAccountingHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, namespace string) {
	t.Helper()

	// Get Accounting CR
	accounting := &slinkyv1beta1.Accounting{}

	accountingKey := crclient.ObjectKey{
		Namespace: namespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, accountingKey, accounting)
	require.NoError(t, err, "failed to Get() accounting using accounting-runtime client")

	accountingUID := accounting.UID

	// Get Accounting StatefulSet using accounting CR
	statefulSetKey := accounting.Key()
	statefulSet := &appsv1.StatefulSet{}
	err = crClient.Get(ctx, statefulSetKey, statefulSet)
	require.NoError(t, err, "failed to Get() statefulset using controller-runtime client")

	// Confirm ownership of controller statefulset
	for _, owner := range statefulSet.OwnerReferences {
		require.Equal(t, accountingUID, owner.UID, "dubious ownership of statefulset: %v", statefulSet)
	}

	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(statefulSet, func(object k8s.Object) int32 {
		return object.(*appsv1.StatefulSet).Status.ReadyReplicas
	}, *statefulSet.Spec.Replicas))
	if err != nil {
		_ = crClient.Get(ctx, accountingKey, accounting)
		t.Fatalf(
			"timed out waiting for accounting StatefulSet %s/%s to reach %d ready replicas: %v; Accounting status: %s; StatefulSet status: %s",
			statefulSet.Namespace,
			statefulSet.Name,
			*statefulSet.Spec.Replicas,
			err,
			test.StatusJSON(accounting.Status),
			test.StatusJSON(statefulSet.Status),
		)
	}
}

func testSlurmAccounting(namespace string) types.Feature {
	return features.New("Assess the functionality of the Slurm Accounting").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Controller can contact accounting", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("Sacctmgr has cluster entry", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "show", "cluster", "format=cluster%30", "-n"}

			cmd := exec.Command(command, args...)
			output, err := cmd.Output()
			require.NoError(t, err, "sacctmgr show cluster returned non-zero error code")
			expectedClusterName := namespace + "_slurm"
			require.Equal(t, expectedClusterName, strings.TrimSpace(string(output)), "clustername in slurmdbd does not match expected %s", expectedClusterName)

			return ctx
		}).
		Assess("Sacctmgr add account", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "add", "account", "name=test", "-i"}
			var wants string
			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 40*time.Second, 5*time.Second)

			args = []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "show", "account", "name=test", "-n", "format=account"}
			wants = "test"

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 40*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("Sacctmgr add user", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "add", "user", "account=test", "name=testuser", "-i"}

			var wants string
			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 40*time.Second, 5*time.Second)

			args = []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "show", "user", "name=testuser", "-n", "format=user"}
			wants = "testuser"

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 40*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("Sacctmgr delete account", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "delete", "account", "test", "-i"}

			cmd := exec.Command(command, args...)
			_, err := cmd.Output()
			require.NoError(t, err, "sacctmgr add account returned non-zero error code")

			args = []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "show", "account", "name=test", "-n", "format=account"}
			cmd = exec.Command(command, args...)
			output, err := cmd.Output()
			require.NoError(t, err, "sacctmgr show account returned non-zero error code")
			require.NotEqual(t, "test", strings.TrimSpace(string(output)), "account test was not deleted from slurmdbd")

			return ctx
		}).Feature()
}

// LoginSet tests

func checkLoginSetHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, namespace string) {
	t.Helper()

	// Get LoginSet CR
	loginSet := &slinkyv1beta1.LoginSet{}

	loginSetKey := crclient.ObjectKey{
		Namespace: namespace,
		Name:      "slurm-login-slinky",
	}

	err := crClient.Get(ctx, loginSetKey, loginSet)
	require.NoError(t, err, "failed to Get() loginSet using controller-runtime client")

	loginSetUID := loginSet.UID

	// Get loginSet Deployment using loginSet CR
	deploymentKey := loginSet.Key()
	deployment := &appsv1.Deployment{}
	err = crClient.Get(ctx, deploymentKey, deployment)
	require.NoError(t, err, "failed to Get() deployment using controller-runtime client")

	// Confirm ownership of loginSet deployment
	for _, owner := range deployment.OwnerReferences {
		require.Equal(t, loginSetUID, owner.UID, "dubious ownership of deployment: %v", deployment)
	}

	// Check whether loginSet deployment is healthy
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, *deployment.Spec.Replicas))
	if err != nil {
		_ = crClient.Get(ctx, loginSetKey, loginSet)
		t.Fatalf(
			"timed out waiting for LoginSet Deployment %s/%s to reach %d ready replicas: %v; LoginSet status: %s; Deployment status: %s",
			deployment.Namespace,
			deployment.Name,
			*deployment.Spec.Replicas,
			err,
			test.StatusJSON(loginSet.Status),
			test.StatusJSON(deployment.Status),
		)
	}
}
