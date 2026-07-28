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

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/test"
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
)

// Dependency Component Health Checks

func checkMariaDBHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	// Get MariaDB CR

	mariadb := &mariadbv1alpha1.MariaDB{}

	mariadbKey := crclient.ObjectKey{
		Namespace: test.SlurmNamespace,
		Name:      "mariadb",
	}

	err := crClient.Get(ctx, mariadbKey, mariadb)
	require.NoError(t, err, "failed to Get() mariadb using controller-runtime client")

	// Get every StatefulSet
	statefulSetList := appsv1.StatefulSetList{}
	err = crClient.List(ctx, &statefulSetList)
	require.NoError(t, err, "failed to List() StatefulSets using controller-runtime client")

	// Build a list of StatefulSets owned by this MariaDB CR
	ownedStatefulSets := appsv1.StatefulSetList{}
	for _, statefulSet := range statefulSetList.Items {
		for _, owner := range statefulSet.OwnerReferences {
			if owner.UID == mariadb.UID {
				ownedStatefulSets.Items = append(ownedStatefulSets.Items, statefulSet)
			}
		}
	}

	// Get MariaDB StatefulSet using CR
	for _, statefulSet := range ownedStatefulSets.Items {
		err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(&statefulSet, func(object k8s.Object) int32 {
			return object.(*appsv1.StatefulSet).Status.ReadyReplicas
		}, *statefulSet.Spec.Replicas))
		require.NoError(t, err, "timed out waiting for StatefulSet %v to reach a ready state", statefulSet.Name)
	}

	return ctx
}

// Slinky Component Health Checks

// Controller tests

func checkControllerHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config) {
	// Get Controller CR
	controller := &slinkyv1beta1.Controller{}

	controllerKey := crclient.ObjectKey{
		Namespace: test.SlurmNamespace,
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
	require.NoError(t, err, "timed out waiting for StatefulSet %v to reach a ready state", statefulSet.Name)
}

func testSlurmController() types.Feature {
	return features.New("Assess the functionality of the Slurm controller").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("slurmctld is responsive", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "scontrol", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 16, time.Duration(5*time.Second))

			return ctx
		}).
		Assess("slurm controller can resolve nodeset by hostname", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			for retry := range 16 {
				nodeInfo, err := test.GetSlurmNodeInfo("slinky-0")
				if err != nil && retry == 15 {
					t.Fatalf("failed to execute command: %v", err)
				}

				if nodeInfo["NodeAddr"] == "" && retry == 15 {
					t.Fatalf("Error resolving hostname for slurm node slinky-0")
				}

				command := "kubectl"
				args := []string{
					"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--",
					"getent", "hosts", nodeInfo["NodeAddr"],
				}

				cmd := exec.Command(command, args...)
				output, err := cmd.Output()
				if err != nil && retry == 15 {
					t.Fatalf("Failed to resolve nodeset by hostname. getent hosts returned: %v", output)
				}

				split_output := strings.Split(string(output), " ")
				if len(split_output) <= 1 && retry == 15 {
					t.Fatalf("Failed to resolve nodeset by hostname. getent hosts returned: %v", output)
				}

				if strings.HasPrefix(strings.TrimSpace(split_output[len(split_output)-1]), "slinky-0") {
					break
				}

				time.Sleep(time.Second * 5)
			}

			return ctx
		}).
		Assess("job launch & execution succeeds (srun)", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "srun", "--immediate=10", "-K", "-Q", "--time=0:15", "hostname"}
			wants := "slinky-0"

			cleanup_command := "kubectl"
			cleanup_args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"}

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 16, time.Duration(5*time.Second))

			return ctx
		}).Feature()
}

// RestAPI tests

func checkRestAPIHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config) {
	// Get RestAPI CR
	restapi := &slinkyv1beta1.RestApi{}

	restapiKey := crclient.ObjectKey{
		Namespace: test.SlurmNamespace,
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
	require.NoError(t, err, "timed out waiting for Deployment %v to reach a ready state", deployment.Name)
}

func testSlurmRestAPI(withAccounting bool) types.Feature {
	return features.New("Assess the functionality of the Slurm RestAPI").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("slurmrestd container args match expectations", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"get", "deployment", "-n", "slurm", "slurm-restapi", "-o", `jsonpath="{.spec.template.spec.containers[0].args}"`}
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
	nodeset := &slinkyv1beta1.NodeSet{}

	for retry := range 16 {

		err := crClient.Get(ctx, nodesetKey, nodeset)
		require.NoError(t, err, "failed to Get() NodeSet using controller-runtime client")

		if *nodeset.Spec.Replicas == nodeset.Status.AvailableReplicas {
			break
		}

		if retry == 15 {
			t.Fatalf("Timed out waiting for NodeSet replicas to become ready. \nDesired replicas: %d \nReady replicas: %d", *nodeset.Spec.Replicas, nodeset.Status.AvailableReplicas)
		}

		time.Sleep(5 * time.Second)
	}
}

func testSlurmNodeSet() types.Feature {
	return features.New("Assess the functionality of the Slurm NodeSet").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Nodeset can contact controller", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-worker-slinky-0", "--", "scontrol", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 48, time.Duration(5*time.Second))

			return ctx
		}).
		Assess("NodeSet is idle", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-worker-slinky-0", "--", "sinfo", "-N", "-n", "slinky-0", "--Format=StateLong", "-h"}
			wants := "idle"

			cleanup_command := "kubectl"
			cleanup_args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"}

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 16, time.Duration(5*time.Second))

			return ctx
		}).
		Assess("NodeSet scale-up functions", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "Failed to get new controller-runtime client")

			nodesetKey := crclient.ObjectKey{
				Namespace: test.SlurmNamespace,
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
			for retry := range 16 {
				nodeInfo, err := test.GetSlurmNodeInfo("slinky-1")
				if err != nil && retry == 15 {
					t.Fatalf("failed to execute command: %v", err)
				}

				if nodeInfo["NodeAddr"] == "" && retry == 15 {
					t.Fatalf("Error resolving hostname for slurm node slinky-1")
				}

				command := "kubectl"
				args := []string{
					"exec", "-n", test.SlurmNamespace, "slurm-worker-slinky-0", "--",
					"getent", "hosts", nodeInfo["NodeAddr"],
				}

				cmd := exec.Command(command, args...)
				output, err := cmd.Output()
				if err != nil && retry == 15 {
					t.Fatalf("Failed to resolve nodeset by hostname. getent hosts returned: %v", output)
				}

				split_output := strings.Split(string(output), " ")
				if len(split_output) <= 1 && retry == 15 {
					t.Fatalf("Failed to resolve nodeset by hostname. getent hosts returned: %v", output)
				}

				if strings.HasPrefix(strings.TrimSpace(split_output[len(split_output)-1]), "slinky-1") {
					break
				}

				time.Sleep(time.Second * 5)
			}

			return ctx
		}).
		Assess("NodeSet scale-down functions", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "Failed to get new controller-runtime client")

			nodesetKey := crclient.ObjectKey{
				Namespace: test.SlurmNamespace,
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

func testSlurmJWTKeyRotation() types.Feature {
	return features.New("Assess Slurm JWT signing key rotation").
		Assess("referenced JWT Secret updates refresh the Slurm client", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			controller := &slinkyv1beta1.Controller{}
			controllerKey := crclient.ObjectKey{
				Namespace: test.SlurmNamespace,
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
				Namespace: test.SlurmNamespace,
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
				Namespace: test.SlurmNamespace,
				Name:      "slurm-worker-slinky",
			}
			nodeset := &slinkyv1beta1.NodeSet{}
			require.NoError(t, crClient.Get(ctx, nodesetKey, nodeset), "failed to get NodeSet")

			var replicas int32 = 2
			nodeset.Spec.Replicas = &replicas
			require.NoError(t, crClient.Update(ctx, nodeset), "failed to scale NodeSet after JWT key rotation")
			checkNodeSetReplicas(crClient, ctx, t, config, nodesetKey)

			test.RetryCommand(
				ctx,
				t,
				"kubectl",
				[]string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sinfo", "-N", "-n", "slinky-1", "--Format=StateLong", "-h"},
				"idle",
				"",
				nil,
				16,
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

func checkAccountingHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config) {
	// Get Accounting CR
	accounting := &slinkyv1beta1.Accounting{}

	accountingKey := crclient.ObjectKey{
		Namespace: test.SlurmNamespace,
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
	require.NoError(t, err, "timed out waiting for StatefulSet %v to reach a ready state", statefulSet.Name)
}

func testSlurmAccounting() types.Feature {
	return features.New("Assess the functionality of the Slurm Accounting").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Controller can contact accounting", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 16, time.Duration(5*time.Second))

			return ctx
		}).
		Assess("Sacctmgr has cluster entry", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "show", "cluster", "format=cluster%30", "-n"}

			cmd := exec.Command(command, args...)
			output, err := cmd.Output()
			require.NoError(t, err, "sacctmgr show cluster returned non-zero error code")
			require.Equal(t, "slurm_slurm", strings.TrimSpace(string(output)), "clustername in slurmdbd does not match expected slurm_slurm")

			return ctx
		}).
		Assess("Sacctmgr add account", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "add", "account", "name=test", "-i"}
			var wants string
			var cleanup_command string
			var cleanup_args []string

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 8, time.Duration(5*time.Second))

			args = []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "show", "account", "name=test", "-n", "format=account"}
			wants = "test"

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 8, time.Duration(5*time.Second))

			return ctx
		}).
		Assess("Sacctmgr add user", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "add", "user", "account=test", "name=testuser", "-i"}

			var wants string
			var cleanup_command string
			var cleanup_args []string

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 8, time.Duration(5*time.Second))

			args = []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "show", "user", "name=testuser", "-n", "format=user"}
			wants = "testuser"

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 8, time.Duration(5*time.Second))

			return ctx
		}).
		Assess("Sacctmgr delete account", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "delete", "account", "test", "-i"}

			cmd := exec.Command(command, args...)
			_, err := cmd.Output()
			require.NoError(t, err, "sacctmgr add account returned non-zero error code")

			args = []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "show", "account", "name=test", "-n", "format=account"}
			cmd = exec.Command(command, args...)
			output, err := cmd.Output()
			require.NoError(t, err, "sacctmgr show account returned non-zero error code")
			require.NotEqual(t, "test", strings.TrimSpace(string(output)), "account test was not deleted from slurmdbd")

			return ctx
		}).Feature()
}

// LoginSet tests

func checkLoginSetHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config) {
	// Get LoginSet CR
	loginSet := &slinkyv1beta1.LoginSet{}

	loginSetKey := crclient.ObjectKey{
		Namespace: test.SlurmNamespace,
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
	require.NoError(t, err, "timed out waiting for Deployment %v to reach a ready state", deployment.Name)
}
