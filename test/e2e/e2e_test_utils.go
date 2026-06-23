// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/test"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"
)

func getFeaturesFromConfig(install bool, test bool, config test.SlurmInstallationConfig, beforeSteps []types.Feature) []types.Feature {
	steps := beforeSteps

	if install {
		steps = append(steps, installSlurm(config))
	}
	if test {

		steps = append(steps, testSlurmController())
		steps = append(steps, testSlurmNodeSet())

		if config.Accounting {
			steps = append(steps, testSlurmAccounting())
			steps = append(steps, testSlurmAccountUserCRDs())
		}
	}

	if install {
		steps = append(steps, uninstallSlurm())

	}

	return steps
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

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 16, time.Duration(5*time.Second))

			return ctx
		}).
		Assess("NodeSet is idle", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-worker-slinky-0", "--", "sinfo", "-N", "-n", "slinky-0", "-p", "slinky", "--Format=StateLong", "-h"}
			wants := "idle"

			cleanup_command := "kubectl"
			cleanup_args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"}

			test.RetryCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 16, time.Duration(5*time.Second))

			return ctx
		}).
		Assess("NodeSet scale-up functions", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			crClient, err := GetControllerRuntimeClient(config)
			if err != nil {
				t.Fatalf("Failed to get new controller-runtime client: %v", err)
			}

			nodesetKey := crclient.ObjectKey{
				Namespace: test.SlurmNamespace,
				Name:      "slurm-worker-slinky",
			}
			nodeset := &slinkyv1beta1.NodeSet{}
			err = crClient.Get(ctx, nodesetKey, nodeset)
			if err != nil {
				t.Fatal("failed to Get() NodeSet using controller-runtime client")
			}

			var replicas int32 = 2
			nodeset.Spec.Replicas = &replicas

			err = crClient.Update(ctx, nodeset)
			if err != nil {
				t.Fatal("failed to Update() NodeSet using controller-runtime client")
			}

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
			if err != nil {
				t.Fatalf("Failed to get new controller-runtime client: %v", err)
			}

			nodesetKey := crclient.ObjectKey{
				Namespace: test.SlurmNamespace,
				Name:      "slurm-worker-slinky",
			}
			nodeset := &slinkyv1beta1.NodeSet{}
			err = crClient.Get(ctx, nodesetKey, nodeset)
			if err != nil {
				t.Fatal("failed to Get() NodeSet using controller-runtime client")
			}

			var replicas int32 = 1
			nodeset.Spec.Replicas = &replicas

			err = crClient.Update(ctx, nodeset)
			if err != nil {
				t.Fatal("failed to Update() NodeSet using controller-runtime client")
			}

			checkNodeSetReplicas(crClient, ctx, t, config, nodesetKey)

			return ctx
		}).Feature()
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
			if err != nil {
				t.Fatal("sacctmgr show cluster returned non-zero error code")
			}

			if strings.TrimSpace(string(output)) != "slurm_slurm" {
				t.Fatalf("Clustername in slurmdbd %s does not match expected slurm_slurm", string(output))
			}

			return ctx
		}).
		Assess("Sacctmgr add account", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "add", "account", "cluster=slurm_slurm", "name=test", "-i"}

			cmd := exec.Command(command, args...)
			_, err := cmd.Output()
			if err != nil {
				t.Fatal("sacctmgr add account returned non-zero error code")
			}

			args = []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "show", "account", "name=test", "-n", "format=account"}
			cmd = exec.Command(command, args...)
			output, err := cmd.Output()
			if err != nil {
				t.Fatal("sacctmgr show account returned non-zero error code")
			}

			if strings.TrimSpace(string(output)) != "test" {
				t.Fatal("Account test does not exist in slurmdbd")
			}

			return ctx
		}).
		Assess("Sacctmgr add user", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "add", "user", "cluster=slurm_slurm", "account=test", "name=testuser", "-i"}

			cmd := exec.Command(command, args...)
			_, err := cmd.Output()
			if err != nil {
				t.Fatal("sacctmgr add user returned non-zero error code")
			}

			args = []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "show", "user", "name=testuser", "-n", "format=user"}
			cmd = exec.Command(command, args...)
			output, err := cmd.Output()
			if err != nil {
				t.Fatal("sacctmgr show user returned non-zero error code")
			}

			if strings.TrimSpace(string(output)) != "testuser" {
				t.Fatal("User testuser does not exist in slurmdbd")
			}

			return ctx
		}).
		Assess("Sacctmgr delete account", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "delete", "account", "test", "-i"}

			cmd := exec.Command(command, args...)
			_, err := cmd.Output()
			if err != nil {
				t.Fatal("sacctmgr add account returned non-zero error code")
			}

			args = []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "show", "account", "name=test", "-n", "format=account"}
			cmd = exec.Command(command, args...)
			output, err := cmd.Output()
			if err != nil {
				t.Fatal("sacctmgr show account returned non-zero error code")
			}

			if strings.TrimSpace(string(output)) == "test" {
				t.Fatal("Account test was not deleted from slurmdbd")
			}

			return ctx
		}).Feature()
}

// testSlurmAccountUserCRDs exercises the Account and User CRDs end-to-end:
// it creates the CRs through the operator, waits for them to become Ready,
// verifies the entities land in slurmdbd, and confirms cleanup on deletion.
func testSlurmAccountUserCRDs() types.Feature {
	accountKey := crclient.ObjectKey{Namespace: test.SlurmNamespace, Name: "research"}
	userKey := crclient.ObjectKey{Namespace: test.SlurmNamespace, Name: "alice"}

	return features.New("Assess the Account and User CRDs").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			if err != nil {
				return ctx
			}
			// Best-effort cleanup; ignore errors (resources may already be gone).
			_ = crClient.Delete(ctx, &slinkyv1beta1.User{ObjectMeta: metav1.ObjectMeta{Name: userKey.Name, Namespace: userKey.Namespace}})
			_ = crClient.Delete(ctx, &slinkyv1beta1.Account{ObjectMeta: metav1.ObjectMeta{Name: accountKey.Name, Namespace: accountKey.Namespace}})
			return ctx
		}).
		Assess("Account CR becomes Ready", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			if err != nil {
				t.Fatalf("Failed to get new controller-runtime client: %v", err)
			}

			account := &slinkyv1beta1.Account{
				ObjectMeta: metav1.ObjectMeta{Name: accountKey.Name, Namespace: accountKey.Namespace},
				Spec: slinkyv1beta1.AccountSpec{
					ControllerRef:  corev1.LocalObjectReference{Name: "slurm"},
					AccountName:    "research",
					Description:    "E2E research account",
					Organization:   "acme",
					ParentAccount:  ptr.To("root"),
					DeletionPolicy: slinkyv1beta1.DeletionPolicyDelete,
					Limits: slinkyv1beta1.AssociationLimits{
						MaxJobs: ptr.To(int32(50)),
					},
				},
			}
			if err := crClient.Create(ctx, account); err != nil {
				t.Fatalf("failed to Create() Account: %v", err)
			}

			waitForReadyCondition(ctx, t, crClient, account, accountKey, func() []metav1.Condition {
				return account.Status.Conditions
			})

			return ctx
		}).
		Assess("Account exists in slurmdbd", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "show", "account", "name=research", "-n", "format=account"}
			wants := "research"

			var cleanupCommand string
			var cleanupArgs []string

			test.RetryCommand(ctx, t, command, args, wants, cleanupCommand, cleanupArgs, 16, time.Duration(5*time.Second))

			return ctx
		}).
		Assess("User CR becomes Ready", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			if err != nil {
				t.Fatalf("Failed to get new controller-runtime client: %v", err)
			}

			user := &slinkyv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{Name: userKey.Name, Namespace: userKey.Namespace},
				Spec: slinkyv1beta1.UserSpec{
					ControllerRef:  corev1.LocalObjectReference{Name: "slurm"},
					UserName:       "alice",
					AdminLevel:     slinkyv1beta1.AdminLevelNone,
					DefaultAccount: "research",
					DeletionPolicy: slinkyv1beta1.DeletionPolicyDelete,
					Associations: []slinkyv1beta1.UserAssociation{
						{Account: "research"},
					},
				},
			}
			if err := crClient.Create(ctx, user); err != nil {
				t.Fatalf("failed to Create() User: %v", err)
			}

			waitForReadyCondition(ctx, t, crClient, user, userKey, func() []metav1.Condition {
				return user.Status.Conditions
			})

			return ctx
		}).
		Assess("User and association exist in slurmdbd", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			command := "kubectl"
			args := []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "show", "user", "name=alice", "-n", "format=user"}
			wants := "alice"

			var cleanupCommand string
			var cleanupArgs []string

			test.RetryCommand(ctx, t, command, args, wants, cleanupCommand, cleanupArgs, 16, time.Duration(5*time.Second))

			args = []string{"exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--", "sacctmgr", "show", "assoc", "where", "user=alice", "account=research", "-n", "format=account"}
			test.RetryCommand(ctx, t, command, args, "research", cleanupCommand, cleanupArgs, 16, time.Duration(5*time.Second))

			return ctx
		}).
		Assess("Deleting CRs removes entities from slurmdbd", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			if err != nil {
				t.Fatalf("Failed to get new controller-runtime client: %v", err)
			}

			user := &slinkyv1beta1.User{ObjectMeta: metav1.ObjectMeta{Name: userKey.Name, Namespace: userKey.Namespace}}
			if err := crClient.Delete(ctx, user); err != nil {
				t.Fatalf("failed to Delete() User: %v", err)
			}
			waitForObjectDeleted(ctx, t, crClient, &slinkyv1beta1.User{}, userKey)

			account := &slinkyv1beta1.Account{ObjectMeta: metav1.ObjectMeta{Name: accountKey.Name, Namespace: accountKey.Namespace}}
			if err := crClient.Delete(ctx, account); err != nil {
				t.Fatalf("failed to Delete() Account: %v", err)
			}
			waitForObjectDeleted(ctx, t, crClient, &slinkyv1beta1.Account{}, accountKey)

			// Confirm the account is gone from slurmdbd.
			for retry := range 16 {
				cmd := exec.Command("kubectl", "exec", "-n", test.SlurmNamespace, "slurm-controller-0", "--",
					"sacctmgr", "show", "account", "name=research", "-n", "format=account")
				output, err := cmd.Output()
				if err != nil && retry == 15 {
					t.Fatalf("sacctmgr show account returned error: %v", err)
				}
				if strings.TrimSpace(string(output)) == "" {
					return ctx
				}
				if retry == 15 {
					t.Fatalf("Account research was not deleted from slurmdbd, got: %q", strings.TrimSpace(string(output)))
				}
				time.Sleep(5 * time.Second)
			}

			return ctx
		}).Feature()
}

// waitForReadyCondition polls the given object until its Ready condition is
// True, re-fetching it on each attempt.
func waitForReadyCondition(ctx context.Context, t *testing.T, crClient crclient.Client, obj crclient.Object, key crclient.ObjectKey, conditions func() []metav1.Condition) {
	for retry := range 24 {
		if err := crClient.Get(ctx, key, obj); err != nil {
			t.Fatalf("failed to Get() %T %s: %v", obj, key.Name, err)
		}
		if meta.IsStatusConditionTrue(conditions(), "Ready") {
			return
		}
		if retry == 23 {
			t.Fatalf("timed out waiting for %T %s Ready=True; conditions: %+v", obj, key.Name, conditions())
		}
		time.Sleep(5 * time.Second)
	}
}

// waitForObjectDeleted polls until the given object no longer exists, allowing
// finalizer-driven Slurm-side cleanup to complete.
func waitForObjectDeleted(ctx context.Context, t *testing.T, crClient crclient.Client, obj crclient.Object, key crclient.ObjectKey) {
	for retry := range 24 {
		err := crClient.Get(ctx, key, obj)
		if crclient.IgnoreNotFound(err) != nil {
			t.Fatalf("failed to Get() %T %s: %v", obj, key.Name, err)
		}
		if err != nil {
			return // NotFound: object deleted
		}
		if retry == 23 {
			t.Fatalf("timed out waiting for %T %s to be deleted", obj, key.Name)
		}
		time.Sleep(5 * time.Second)
	}
}

func GetControllerRuntimeClient(config *envconf.Config) (crclient.Client, error) {
	var scheme = k8sruntime.NewScheme()
	err := slinkyv1beta1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = appsv1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = corev1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	err = mariadbv1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	return klient.NewControllerRuntimeClient(config.Client().RESTConfig(), scheme)
}
