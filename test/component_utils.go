// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

func DoSlurmInstall(ctx context.Context, t *testing.T, config *envconf.Config, slurmConfig SlurmInstallationConfig) context.Context {
	manager := helm.New(config.KubeconfigFile())

	setValuesFile := fmt.Sprintf("--values %s/helm/slurm/values.yaml", Basepath)
	enableNodeset := "--set nodesets.slinky.enabled=true"
	enablePartition := "--set partitions.all.enabled=true"

	opts := []helm.Option{}
	opts = append(
		opts,
		helm.WithName("slurm"),
		helm.WithNamespace(SlurmNamespace),
		helm.WithChart(Basepath+"helm/slurm"),
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
		opts = append(opts, helm.WithArgs("--set 'loginsets.slinky.login.image.repository=ghcr.io/slinkyproject/login-pyxis'"))
		opts = append(opts, helm.WithArgs("--set 'loginsets.slinky.securityContext.privileged=true'"))
		opts = append(opts, helm.WithArgs("--set 'nodesets.slinky.slurmd.image.repository=ghcr.io/slinkyproject/slurmd-pyxis'"))
	}

	err := manager.RunInstall(opts...)
	require.NoError(t, err, "failed to invoke helm install operation due to an error")

	return ctx
}

func CheckDeploymentStatus(ctx context.Context, t *testing.T, config *envconf.Config, deploymentName string, deploymentNamespace string) context.Context {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: deploymentNamespace,
		},
	}
	err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, 1))
	require.NoError(t, err, "failed waiting for the %s deployment to reach a ready state", deploymentName)

	return ctx
}

func DoUninstallHelmChart(ctx context.Context, t *testing.T, config *envconf.Config, chartName string, chartNamespace string) context.Context {
	manager := helm.New(config.KubeconfigFile())

	err := manager.RunUninstall(
		helm.WithName(chartName),
		helm.WithNamespace(chartNamespace),
		helm.WithWait(),
		helm.WithTimeout("5m"),
	)
	require.NoError(t, err, "failed to invoke helm uninstall %s due to an error", chartName)

	return ctx
}
