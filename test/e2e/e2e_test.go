// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/types"
	"sigs.k8s.io/e2e-framework/support/kind"

	"helm.sh/helm/v3/pkg/action"
	// mariadb "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
)

var (
	testenv env.Environment
	testUID string
)

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := envconf.RandomName("test-e2e", 16)
	basepath = getBasePath()

	// Build images for Slurm-operator and Slurm-operator-webhook
	testUID = envconf.RandomName("testing", 16)
	operatorName := "ghcr.io/slinkyproject/slurm-operator:" + testUID
	webhookName := "ghcr.io/slinkyproject/slurm-operator-webhook:" + testUID
	err := BuildOperatorImages(operatorName, webhookName)
	if err != nil {
		fmt.Printf("Failed to build images for Slurm-operator: %v", err)
		os.Exit(1)
	}

	// Build the slurm-operator-crds Helm chart
	slurmOperatorCRDs := action.Package{
		DependencyUpdate: true,
		Destination:      basepath + "helm/slurm-operator/charts",
	}
	_, err = slurmOperatorCRDs.Run(basepath+"helm/slurm-operator-crds", nil)
	if err != nil {
		fmt.Printf("Failed to build Helm chart for Slurm-operator: %v", err)
		os.Exit(1)
	}

	// Use pre-defined environment funcs to create a kind cluster prior to test run
	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),
		envfuncs.LoadDockerImageToCluster(kindClusterName, operatorName),
		envfuncs.LoadDockerImageToCluster(kindClusterName, webhookName),
		envfuncs.CreateNamespace("slinky"),
		envfuncs.CreateNamespace("slurm"),
		envfuncs.CreateNamespace("cert-manager"),
		envfuncs.CreateNamespace("mariadb"),
		envfuncs.CreateNamespace("prometheus"),
	)

	// Use pre-defined environment funcs to teardown kind cluster after tests
	testenv.Finish(
		envfuncs.DeleteNamespace("slinky"),
		envfuncs.DeleteNamespace("slurm"),
		envfuncs.DeleteNamespace("cert-manager"),
		envfuncs.DeleteNamespace("mariadb"),
		envfuncs.DeleteNamespace("prometheus"),
		envfuncs.DestroyCluster(kindClusterName),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}

func TestHelmInstallation(t *testing.T) {
	const slinkyNamespace = "slinky"
	const slurmNamespace = "slurm"

	tests := []struct {
		name  string
		steps []types.Feature
	}{
		{
			name: "Install slurm-operator",
			steps: []types.Feature{
				installCertMgr(),
				installSlurmOperatorCRDS(slinkyNamespace),
				installSlurmOperator(testUID, slinkyNamespace),
			},
		},
		{
			name: "Install Slurm",
			steps: []types.Feature{
				installSlurm(slurmNamespace, false, false, false),
				scontrolPing(slurmNamespace),
				srun(slurmNamespace),
				uninstallSlurm(slurmNamespace),
			},
		},
		{
			name: "Install Slurm with login",
			steps: []types.Feature{
				installSlurm(slurmNamespace, false, true, false),
				scontrolPing(slurmNamespace),
				srun(slurmNamespace),
				uninstallSlurm(slurmNamespace),
			},
		},
		{
			name: "Install Slurm with metrics",
			steps: []types.Feature{
				installPrometheus(),
				installSlurm(slurmNamespace, false, false, true),
				scontrolPing(slurmNamespace),
				srun(slurmNamespace),
				uninstallSlurm(slurmNamespace),
			},
		},
		{
			name: "Install Slurm with accounting",
			steps: []types.Feature{
				installMariadbOperator(),
				applyMariaDBYaml(slurmNamespace),
				installSlurm(slurmNamespace, true, false, false),
				scontrolPing(slurmNamespace),
				srun(slurmNamespace),
				uninstallSlurm(slurmNamespace),
			},
		},
		{
			name: "Uninstall slurm-operator",
			steps: []types.Feature{
				uninstallSlurmOperator(slinkyNamespace),
				uninstallSlurmOperatorCRDs(slinkyNamespace),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = testenv.Test(t, tt.steps...)
		})
	}
}
