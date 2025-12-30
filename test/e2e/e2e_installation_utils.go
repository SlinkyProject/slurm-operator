// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"os/exec"
	"testing"

	"github.com/SlinkyProject/slurm-operator/test"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"
)

// Dependency Installation

func installCertMgr() types.Feature {
	return features.New("Helm install cert-manager").
		Setup(test.DoCertMgrInstall).
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

func installMariadbOperator() types.Feature {
	return features.New("Helm install mariadb-operator").
		Setup(test.DoMariaDBInstall).
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
	return features.New("Create MariaDB instance for Slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			path := test.Basepath + "/hack/resources/mariadb.yaml"

			cmd := exec.Command("kubectl", "apply", "-f", path)
			_, err := cmd.Output()
			if err != nil {
				t.Fatalf("failed running 'kubectl apply -f %s /hack/resources/mariadb.yaml: %v", test.Basepath, err)
			}

			return ctx
		}).
		Assess("Pod mariadb-0 running successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			if err != nil {
				t.Fatalf("Failed to get new controller-runtime client: %v", err)
			}

			checkMariaDBHealth(crClient, ctx, t, config)

			return ctx
		}).Feature()
}

func installPrometheus() types.Feature {
	return features.New("Helm install prometheus").
		Setup(test.DoPrometheusInstall).
		Assess("prometheus deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "prometheus-kube-prometheus-operator", "prometheus")
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
			if err != nil {
				t.Fatalf("Failed to get new controller-runtime client: %v", err)
			}

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

func installSlurmOperatorCRDS() types.Feature {
	return features.New("Helm install slurm-operator-crds").
		Setup(test.DoSlurmOperatorCRDInstall).Feature()
}

func installSlurmOperator() types.Feature {
	return features.New("Helm install slurm-operator").
		Setup(test.DoSlurmOperatorInstall).
		Assess("Deployment slurm-operator running successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "slurm-operator", test.SlinkyNamespace)
		}).
		Assess("Deployment slurm-operator-webhook running successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.CheckDeploymentStatus(ctx, t, config, "slurm-operator-webhook", test.SlinkyNamespace)
		}).Feature()
}

// Uninstall Slurm Components

func uninstallSlurm() types.Feature {
	return features.New("Helm uninstall slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.DoUninstallHelmChart(ctx, t, config, "slurm", test.SlurmNamespace)
		}).Feature()
}

func uninstallSlurmOperator() types.Feature {
	return features.New("Helm uninstall slurm-operator").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.DoUninstallHelmChart(ctx, t, config, "slurm-operator", test.SlinkyNamespace)
		}).Feature()
}

func uninstallSlurmOperatorCRDs() types.Feature {
	return features.New("Helm uninstall slurm-operator-crds").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return test.DoUninstallHelmChart(ctx, t, config, "slurm-operator-crds", test.SlinkyNamespace)
		}).Feature()
}
