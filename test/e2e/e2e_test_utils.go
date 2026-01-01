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
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
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
