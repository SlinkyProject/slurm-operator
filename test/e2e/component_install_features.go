// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"testing"

	"github.com/SlinkyProject/slurm-operator/test"
	"github.com/stretchr/testify/require"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"
	"sigs.k8s.io/e2e-framework/third_party/helm"
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

func applyMariaDBYaml() types.Feature {
	return features.New("Ensure MariaDB instance exists for Slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Pod mariadb-0 running successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "Failed to get new controller-runtime client")

			checkMariaDBHealth(crClient, ctx, t, config)

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

			checkControllerHealth(crClient, ctx, t, config)
			return ctx
		}).
		Assess("REST API Deployment is ready", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			checkRestAPIHealth(crClient, ctx, t, config)
			return ctx
		}).
		Assess("NodeSet replicas are available", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			checkNodeSetReplicas(crClient, ctx, t, config, crclient.ObjectKey{
				Namespace: test.SlurmNamespace,
				Name:      "slurm-worker-slinky",
			})
			return ctx
		})

	if slurmConfig.Accounting {
		feature.Assess("Accounting StatefulSet is ready", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			checkAccountingHealth(crClient, ctx, t, config)
			return ctx
		})
	}

	if slurmConfig.Login {
		feature.Assess("LoginSet Deployment is ready", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			checkLoginSetHealth(crClient, ctx, t, config)
			return ctx
		})
	}

	return feature.Feature()
}

func doSlurmInstall(ctx context.Context, t *testing.T, config *envconf.Config, slurmConfig test.SlurmInstallationConfig) context.Context {
	manager := helm.New(config.KubeconfigFile())

	setValuesFile := fmt.Sprintf("--values %s/helm/slurm/values.yaml", test.Basepath)
	enableNodeset := "--set nodesets.slinky.enabled=true"
	enablePartition := "--set partitions.all.enabled=true"

	opts := []helm.Option{}
	opts = append(
		opts,
		helm.WithName("slurm"),
		helm.WithNamespace(test.SlurmNamespace),
		helm.WithChart(test.Basepath+"helm/slurm"),
		helm.WithArgs(setValuesFile, enableNodeset, enablePartition),
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

func uninstallSlurm() types.Feature {
	return features.New("Helm uninstall slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.DoUninstallHelmChart(ctx, t, config, "slurm", test.SlurmNamespace)
		}).Feature()
}
