// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"os"
	"testing"

	"github.com/SlinkyProject/slurm-operator/test"
	"helm.sh/helm/v3/pkg/action"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/types"
	"sigs.k8s.io/e2e-framework/support/kind"
)

// TestMain configures the environment within which all e2e-tests are run
func TestMain(m *testing.M) {
	test.Testenv = env.New()
	kindClusterName := envconf.RandomName("test-e2e", 16)
	test.Basepath = test.GetBasePath()

	// Build images for Slurm-operator and Slurm-operator-webhook
	test.TestUID = envconf.RandomName("testing", 16)
	operatorName := "ghcr.io/slinkyproject/slurm-operator:" + test.TestUID
	webhookName := "ghcr.io/slinkyproject/slurm-operator-webhook:" + test.TestUID
	err := test.BuildOperatorImages(operatorName, webhookName)
	if err != nil {
		fmt.Printf("Failed to build images for Slurm-operator: %v", err)
		os.Exit(1)
	}

	// Build the slurm-operator-crds Helm chart
	slurmOperatorCRDs := action.Package{
		DependencyUpdate: true,
		Destination:      test.Basepath + "helm/slurm-operator/charts",
	}
	_, err = slurmOperatorCRDs.Run(test.Basepath+"helm/slurm-operator-crds", nil)
	if err != nil {
		fmt.Printf("Failed to build Helm chart for Slurm-operator: %v", err)
		os.Exit(1)
	}

	// Use pre-defined environment funcs to create a kind cluster prior to test run
	test.Testenv.Setup(
		envfuncs.CreateClusterWithConfig(kind.NewProvider(), kindClusterName, test.Basepath+"hack/kind.yaml"),
		envfuncs.LoadDockerImageToCluster(kindClusterName, operatorName),
		envfuncs.LoadDockerImageToCluster(kindClusterName, webhookName),
		envfuncs.CreateNamespace("slinky"),
		envfuncs.CreateNamespace("slurm"),
		envfuncs.CreateNamespace("cert-manager"),
		envfuncs.CreateNamespace("mariadb"),
		envfuncs.CreateNamespace("prometheus"),
	)

	// Use pre-defined environment funcs to teardown kind cluster after tests
	test.Testenv.Finish(
		envfuncs.DeleteNamespace("slinky"),
		envfuncs.DeleteNamespace("slurm"),
		envfuncs.DeleteNamespace("cert-manager"),
		envfuncs.DeleteNamespace("mariadb"),
		envfuncs.DeleteNamespace("prometheus"),
		envfuncs.DestroyCluster(kindClusterName),
	)

	// launch package tests
	os.Exit(test.Testenv.Run(m))
}

func TestInstallation(t *testing.T) {
	tests := []struct {
		name         string
		install      bool
		test         bool
		dependencies []types.Feature
		config       test.SlurmInstallationConfig
	}{
		{
			name: "Install slurm-operator",
			dependencies: []types.Feature{
				installCertMgr(),
				installSlurmOperatorCRDS(),
				installSlurmOperator(),
			},
		},
		{
			name:    "Install Slurm",
			install: true,
			test:    true,
			config:  test.SlurmInstallationConfig{},
		},
		{
			name:    "Install Slurm with login",
			install: true,
			test:    true,
			config: test.SlurmInstallationConfig{
				Login: true,
			},
		},
		{
			name:    "Install Slurm with metrics",
			install: true,
			test:    true,
			config: test.SlurmInstallationConfig{
				Metrics: true,
			},
		},
		{
			name:    "Install Slurm with accounting",
			install: true,
			test:    true,
			dependencies: []types.Feature{
				installMariadbOperator(),
				applyMariaDBYaml(),
			},
			config: test.SlurmInstallationConfig{
				Accounting: true,
			},
		},
		{
			name:    "Install Slurm with Pyxis and Login",
			install: true,
			test:    true,
			config: test.SlurmInstallationConfig{
				Pyxis: true,
			},
		},
		{
			name:    "Install Slurm with Pyxis, Login, and Accounting",
			install: true,
			test:    true,
			config: test.SlurmInstallationConfig{
				Pyxis:      true,
				Accounting: true,
			},
		},
		{
			name: "Uninstall slurm-operator",
			dependencies: []types.Feature{
				uninstallSlurmOperator(),
				uninstallSlurmOperatorCRDs(),
			},
		},
	}

	for _, tt := range tests {
		steps := getFeaturesFromConfig(tt.install, tt.test, tt.config, tt.dependencies)

		t.Run(tt.name, func(t *testing.T) {
			_ = test.Testenv.Test(t, steps...)
		})
	}
}
