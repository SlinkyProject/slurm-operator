// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"

	"github.com/SlinkyProject/slurm-operator/test"
	"github.com/stretchr/testify/require"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"
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
	return features.New("Helm install slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.DoSlurmInstall(ctx, t, config, slurmConfig)
		}).
		Assess("Slurm Cluster Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "Failed to get new controller-runtime client")

			checkControllerHealth(crClient, ctx, t, config)
			checkRestAPIHealth(crClient, ctx, t, config)
			checkNodeSetReplicas(crClient, ctx, t, config, crclient.ObjectKey{
				Namespace: test.SlurmNamespace,
				Name:      "slurm-worker-slinky",
			})

			if slurmConfig.Accounting {
				checkAccountingHealth(crClient, ctx, t, config)
			}

			if slurmConfig.Login {
				checkLoginSetHealth(crClient, ctx, t, config)
			}

			return ctx
		}).Feature()
}

// Uninstall Slurm Components

func uninstallSlurm() types.Feature {
	return features.New("Helm uninstall slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.DoUninstallHelmChart(ctx, t, config, "slurm", test.SlurmNamespace)
		}).Feature()
}
