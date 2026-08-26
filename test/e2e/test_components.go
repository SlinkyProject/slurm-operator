// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os/exec"
	"strings"
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
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
		nodeInfo, err := test.GetSlurmNodeInfo(namespace, nodeName)
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

			// NodeSet reconciliation performs authenticated Slurm REST requests.
			// Without the JWT Secret watch, the old token is rejected here and
			// the NodeSet cannot complete its scale-up.
			nodesetKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky",
			}
			nodeset := &slinkyv1beta1.NodeSet{}
			require.NoError(t, crClient.Get(ctx, nodesetKey, nodeset), "failed to get NodeSet")

			var replicas int32 = 2
			nodeset.Spec.Replicas = &replicas
			require.NoError(t, crClient.Update(ctx, nodeset), "failed to scale NodeSet after JWT key rotation")
			checkNodeSetReplicas(crClient, ctx, t, config, nodesetKey)

			test.WaitForCommand(
				ctx,
				t,
				"kubectl",
				[]string{"exec", "-n", namespace, "slurm-controller-0", "--", "sinfo", "-N", "-n", "slinky-1", "--Format=StateLong", "-h"},
				"idle",
				"",
				nil,
				80*time.Second,
				5*time.Second,
			)

			nodeset = &slinkyv1beta1.NodeSet{}
			require.NoError(t, crClient.Get(ctx, nodesetKey, nodeset), "failed to get scaled NodeSet")
			replicas = 1
			nodeset.Spec.Replicas = &replicas
			require.NoError(t, crClient.Update(ctx, nodeset), "failed to restore NodeSet replicas")
			checkNodeSetReplicas(crClient, ctx, t, config, nodesetKey)

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
