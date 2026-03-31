// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

func DoCertMgrInstall(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	manager := helm.New(config.KubeconfigFile())

	err := manager.RunRepo(helm.WithArgs("add", "jetstack", "https://charts.jetstack.io"))
	if err != nil {
		t.Fatal("failed to add jetstack helm chart repo")
	}
	err = manager.RunRepo(helm.WithArgs("update"))
	if err != nil {
		t.Fatal("failed to upgrade helm repo")
	}
	err = manager.RunInstall(helm.WithName("cert-manager"), helm.WithNamespace("cert-manager"),
		helm.WithReleaseName("jetstack/cert-manager"),
		// pinning to a specific version to make sure we will have reproducible executions
		helm.WithVersion("1.19.1"),
		helm.WithArgs("--set 'crds.enabled=true'"),
	)
	if err != nil {
		t.Fatal("failed to install cert-manager Helm chart", err)
	}

	return ctx
}

func DoMariaDBInstall(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	manager := helm.New(config.KubeconfigFile())

	err := manager.RunRepo(helm.WithArgs("add", "mariadb-operator", "https://helm.mariadb.com/mariadb-operator"))
	if err != nil {
		t.Fatal("failed to add mariadb-operator helm chart repo")
	}
	err = manager.RunRepo(helm.WithArgs("update"))
	if err != nil {
		t.Fatal("failed to upgrade helm repo")
	}
	err = manager.RunInstall(helm.WithName("mariadb-operator"), helm.WithNamespace("mariadb"),
		helm.WithReleaseName("mariadb-operator/mariadb-operator"),
		// pinning to a specific version to make sure we will have reproducible executions
		helm.WithVersion("25.10.2"),
		helm.WithArgs("--set 'crds.enabled=true'"),
	)
	if err != nil {
		t.Fatal("failed to install mariadb-operator Helm chart", err)
	}

	return ctx
}

func DoPrometheusInstall(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	manager := helm.New(config.KubeconfigFile())

	err := manager.RunRepo(helm.WithArgs("add", "prometheus-community", "https://prometheus-community.github.io/helm-charts"))
	if err != nil {
		t.Fatal("failed to add prometheus-community helm chart repo")
	}
	err = manager.RunRepo(helm.WithArgs("update"))
	if err != nil {
		t.Fatal("failed to update helm repo")
	}
	err = manager.RunInstall(helm.WithName("prometheus"), helm.WithNamespace("prometheus"),
		helm.WithReleaseName("prometheus-community/kube-prometheus-stack"),
		helm.WithArgs("--set 'installCRDs=true'"),
	)
	if err != nil {
		t.Fatal("failed to install prometheus Helm chart", err)
	}

	return ctx
}

func DoSlurmOperatorCRDInstall(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	manager := helm.New(config.KubeconfigFile())

	err := manager.RunInstall(
		helm.WithName("slurm-operator-crds"),
		helm.WithNamespace(SlinkyNamespace),
		helm.WithChart(Basepath+"helm/slurm-operator-crds"),
		helm.WithWait(),
		helm.WithTimeout("10m"))
	if err != nil {
		t.Fatal("failed to invoke helm install slurm-operator-crds due to an error", err)
	}
	return ctx
}

// splitImageRef splits a container image reference into repository and tag.
// If no tag is present, the tag defaults to "latest".
func splitImageRef(ref string) (repo, tag string) {
	// Handle digests (repo@sha256:...) — keep the full ref as repository.
	if strings.Contains(ref, "@") {
		return ref, ""
	}
	lastColon := strings.LastIndex(ref, ":")
	// If there is no colon, or the colon is part of a registry port (contains '/'),
	// treat the whole string as repository.
	if lastColon == -1 || strings.Contains(ref[lastColon:], "/") {
		return ref, "latest"
	}
	return ref[:lastColon], ref[lastColon+1:]
}

func DoSlurmOperatorInstall(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	manager := helm.New(config.KubeconfigFile())

	opts := []helm.Option{
		helm.WithName("slurm-operator"),
		helm.WithNamespace(SlinkyNamespace),
		helm.WithChart(Basepath + "helm/slurm-operator"),
		helm.WithWait(),
		helm.WithTimeout("10m"),
	}

	opRepo, opTag := splitImageRef(OperatorImage)
	whRepo, whTag := splitImageRef(WebhookImage)

	opts = append(opts,
		helm.WithArgs(fmt.Sprintf("--set operator.image.repository=%s", opRepo)),
		helm.WithArgs(fmt.Sprintf("--set operator.image.tag=%s", opTag)),
		helm.WithArgs(fmt.Sprintf("--set webhook.image.repository=%s", whRepo)),
		helm.WithArgs(fmt.Sprintf("--set webhook.image.tag=%s", whTag)),
	)

	if err := manager.RunInstall(opts...); err != nil {
		t.Fatal("failed to invoke helm install slurm-operator due to an error", err)
	}
	return ctx
}

func DoSlurmInstall(ctx context.Context, t *testing.T, config *envconf.Config, slurmConfig SlurmInstallationConfig) context.Context {
	manager := helm.New(config.KubeconfigFile())

	setValuesFile := fmt.Sprintf("--values %s/helm/slurm/values.yaml", Basepath)

	var err error

	opts := []helm.Option{}
	opts = append(
		opts,
		helm.WithName("slurm"),
		helm.WithNamespace(SlurmNamespace),
		helm.WithChart(Basepath+"helm/slurm"),
		helm.WithArgs(setValuesFile),
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

	err = manager.RunInstall(opts...)

	if err != nil {
		t.Fatal("failed to invoke helm install operation due to an error", err)
	}

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
	if err != nil {
		t.Fatalf("failed waiting for the %s deployment to reach a ready state", deploymentName)
	}

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

	if err != nil {
		t.Fatalf("failed to invoke helm uninstall %s due to an error: %v", chartName, err)
	}

	return ctx
}
