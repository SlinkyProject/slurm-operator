// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"github.com/SlinkyProject/slurm-operator/test"
)

// Dependency Validation

func testCertMgr() types.Feature {
	return features.New("Ensure cert-manager is installed").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("cert-manager Deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "cert-manager", "cert-manager")
		}).
		Assess("cert-manager-cainjector Deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "cert-manager-cainjector", "cert-manager")
		}).
		Assess("cert-manager-webhook deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "cert-manager-webhook", "cert-manager")
		}).Feature()
}

func testMariadbOperator() types.Feature {
	return features.New("Ensure mariadb-operator is installed").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("mariadb-operator deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "mariadb-operator", "mariadb")
		}).
		Assess("mariadb-operator-cert-controller deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "mariadb-operator-cert-controller", "mariadb")
		}).
		Assess("mariadb-operator-webhook deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "mariadb-operator-webhook", "mariadb")
		}).Feature()
}

func applyMariaDBYaml(namespace string) types.Feature {
	return features.New("Ensure MariaDB instance exists for Slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			require.NotEmpty(t, namespace, "MariaDB fixture namespace must not be empty")
			require.NotEqual(t, "slurm", namespace, "MariaDB fixture must not modify the developer namespace")

			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get new controller-runtime client")

			namespaceObject := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespace},
			}
			err = crClient.Create(ctx, namespaceObject)
			require.True(t, err == nil || apierrors.IsAlreadyExists(err), "failed to create namespace %s: %v", namespace, err)

			manifest := filepath.Join(test.Basepath, "hack/resources/mariadb.yaml")
			cmd := exec.CommandContext(ctx, "kubectl", "apply", "--namespace", namespace, "--filename", manifest)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "failed to apply MariaDB fixture in namespace %s: %s", namespace, output)

			return ctx
		}).
		Assess("Pod mariadb-0 running successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get new controller-runtime client")

			checkMariaDBHealth(crClient, ctx, t, config, namespace)

			return ctx
		}).Feature()
}

func testPrometheus() types.Feature {
	return features.New("Ensure prometheus is installed").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("prometheus deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "prometheus-kube-prometheus-operator", "prometheus")
		}).Feature()
}

func testSlurmOperator() types.Feature {
	return features.New("Ensure Slurm-operator is installed").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Deployment slurm-operator running successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "slurm-operator", test.SlinkyNamespace)
		}).
		Assess("Deployment slurm-operator-webhook running successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "slurm-operator-webhook", test.SlinkyNamespace)
		}).Feature()
}

// Slinky Components Installation

func installSlurm(slurmConfig test.SlurmInstallationConfig) types.Feature {
	feature := features.New("Helm install slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return doSlurmInstall(ctx, t, config, slurmConfig)
		}).
		Assess("Controller StatefulSet is ready", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			checkControllerHealth(crClient, ctx, t, config, slurmConfig.Namespace)
			return ctx
		}).
		Assess("REST API Deployment is ready", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			checkRestAPIHealth(crClient, ctx, t, config, slurmConfig.Namespace)
			return ctx
		}).
		Assess("NodeSet replicas are available", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			checkNodeSetReplicas(crClient, ctx, t, config, crclient.ObjectKey{
				Namespace: slurmConfig.Namespace,
				Name:      "slurm-worker-slinky",
			})
			return ctx
		})

	if slurmConfig.Accounting {
		feature.Assess("Accounting StatefulSet is ready", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			checkAccountingHealth(crClient, ctx, t, config, slurmConfig.Namespace)
			return ctx
		})
	}

	if slurmConfig.Login {
		feature.Assess("LoginSet Deployment is ready", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			checkLoginSetHealth(crClient, ctx, t, config, slurmConfig.Namespace)
			return ctx
		})
	}

	return feature.Feature()
}

func doSlurmInstall(ctx context.Context, t *testing.T, config *envconf.Config, slurmConfig test.SlurmInstallationConfig) context.Context {
	require.NotEmpty(t, slurmConfig.Namespace, "Slurm test namespace must not be empty")
	require.NotEqual(t, "slurm", slurmConfig.Namespace, "Slurm tests must not replace the developer release")

	manager := helm.New(config.KubeconfigFile())

	setValuesFile := fmt.Sprintf("--values %s/helm/slurm/values.yaml", test.Basepath)
	enableNodeset := "--set nodesets.slinky.enabled=true"
	enablePartition := "--set partitions.all.enabled=true"

	opts := []helm.Option{}
	opts = append(
		opts,
		helm.WithName("slurm"),
		helm.WithNamespace(slurmConfig.Namespace),
		helm.WithChart(test.Basepath+"helm/slurm"),
		helm.WithArgs(setValuesFile, enableNodeset, enablePartition, "--create-namespace"),
		helm.WithWait(),
		helm.WithTimeout("10m"),
	)

	if slurmConfig.Accounting {
		opts = append(opts, helm.WithArgs("--set 'accounting.enabled=true'"))
	}

	if slurmConfig.Login || slurmConfig.Pyxis {
		opts = append(opts, helm.WithArgs("--set 'loginsets.slinky.enabled=true'"))
	}

	if slurmConfig.Metrics {
		opts = append(opts, helm.WithArgs("--set 'controller.metrics.enabled=true'"))
		opts = append(opts, helm.WithArgs("--set 'controller.metrics.serviceMonitor.enabled=true'"))
	}

	if slurmConfig.Pyxis {
		opts = append(opts, helm.WithArgs(`--set-json 'configFiles={"plugstack.conf":"include /usr/share/pyxis/*"}'`))
		opts = append(opts, helm.WithArgs("--set 'loginsets.slinky.login.image.repository="+test.SlurmImage.Repo+"/login-pyxis'"))
		opts = append(opts, helm.WithArgs("--set 'loginsets.slinky.securityContext.privileged=true'"))
		opts = append(opts, helm.WithArgs("--set 'nodesets.slinky.slurmd.image.repository="+test.SlurmImage.Repo+"/slurmd-pyxis'"))
	}

	if test.SlurmImage.Repo != "" {
		opts = append(opts, helm.WithArgs("--set 'controller.slurmctld.image.repository="+test.SlurmImage.Repo+"/slurmctld'"))
		opts = append(opts, helm.WithArgs("--set 'controller.reconfigure.image.repository="+test.SlurmImage.Repo+"/slurmctld'"))
		opts = append(opts, helm.WithArgs("--set 'restapi.slurmrestd.image.repository="+test.SlurmImage.Repo+"/slurmrestd'"))
		opts = append(opts, helm.WithArgs("--set 'accounting.slurmdbd.image.repository="+test.SlurmImage.Repo+"/slurmdbd'"))
		opts = append(opts, helm.WithArgs("--set 'loginsetDefaults.login.image.repository="+test.SlurmImage.Repo+"/login'"))
		opts = append(opts, helm.WithArgs("--set 'nodesetDefaults.slurmd.image.repository="+test.SlurmImage.Repo+"/slurmd'"))
	}

	if test.SlurmImage.Tag != "" {
		opts = append(opts, helm.WithArgs("--set 'controller.slurmctld.image.tag="+test.SlurmImage.Tag+"'"))
		opts = append(opts, helm.WithArgs("--set 'controller.reconfigure.image.tag="+test.SlurmImage.Tag+"'"))
		opts = append(opts, helm.WithArgs("--set 'restapi.slurmrestd.image.tag="+test.SlurmImage.Tag+"'"))
		opts = append(opts, helm.WithArgs("--set 'accounting.slurmdbd.image.tag="+test.SlurmImage.Tag+"'"))
		opts = append(opts, helm.WithArgs("--set 'loginsetDefaults.login.image.tag="+test.SlurmImage.Tag+"'"))
		opts = append(opts, helm.WithArgs("--set 'nodesetDefaults.slurmd.image.tag="+test.SlurmImage.Tag+"'"))
	}

	err := manager.RunInstall(opts...)
	require.NoError(t, err, "failed to invoke helm install operation due to an error")

	return ctx
}

// Uninstall Slurm Components

func uninstallSlurm(namespace string) types.Feature {
	return features.New("Helm uninstall slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			require.NotEmpty(t, namespace, "Slurm test namespace must not be empty")
			require.NotEqual(t, "slurm", namespace, "Slurm tests must not remove the developer release")
			ctx = test.DoUninstallHelmChart(ctx, t, config, "slurm", namespace)
			return test.DoDeleteNamespace(ctx, t, config, namespace)
		}).Feature()
}
