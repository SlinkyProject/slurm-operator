// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/SlinkyProject/slurm-operator/test"
	"helm.sh/helm/v3/pkg/action"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/types"
	"sigs.k8s.io/e2e-framework/support/kind"
)

// useExistingCluster returns true when E2E_USE_EXISTING_CLUSTER is set to a truthy value.
// When true, the tests target the cluster referenced by the current kubeconfig
// instead of provisioning a disposable kind cluster.
func useExistingCluster() bool {
	v := strings.ToLower(os.Getenv("E2E_USE_EXISTING_CLUSTER"))
	return v == "true" || v == "1" || v == "yes"
}

// TestMain configures the environment within which all e2e-tests are run
func TestMain(m *testing.M) {
	test.Basepath = test.GetBasePath()
	test.TestUID = envconf.RandomName("testing", 16)

	if useExistingCluster() {
		testMainExistingCluster(m)
	} else {
		testMainKindCluster(m)
	}
}

// testMainExistingCluster runs e2e tests against a pre-existing cluster.
//
// Supported environment variables:
//
//	E2E_USE_EXISTING_CLUSTER=true  - required to activate this path
//	E2E_OPERATOR_IMAGE             - operator image already available in the cluster
//	E2E_WEBHOOK_IMAGE              - webhook image already available in the cluster
//	KUBECONFIG                     - path to kubeconfig (standard kubectl resolution when unset)
func testMainExistingCluster(m *testing.M) {
	kubeconfig := os.Getenv("KUBECONFIG")
	test.Testenv = env.NewWithKubeConfig(kubeconfig)

	operatorName := os.Getenv("E2E_OPERATOR_IMAGE")
	webhookName := os.Getenv("E2E_WEBHOOK_IMAGE")

	if operatorName == "" || webhookName == "" {
		// No pre-built images supplied; build them locally.
		operatorName = "ghcr.io/slinkyproject/slurm-operator:" + test.TestUID
		webhookName = "ghcr.io/slinkyproject/slurm-operator-webhook:" + test.TestUID
		if err := test.BuildOperatorImages(operatorName, webhookName); err != nil {
			fmt.Printf("Failed to build images for Slurm-operator: %v", err)
			os.Exit(1)
		}
	}

	// Store image names so downstream helpers can reference them.
	test.OperatorImage = operatorName
	test.WebhookImage = webhookName

	buildHelmChart()

	test.Testenv.Setup(
		envfuncs.CreateNamespace("slinky"),
		envfuncs.CreateNamespace("slurm"),
		envfuncs.CreateNamespace("cert-manager"),
		envfuncs.CreateNamespace("mariadb"),
		envfuncs.CreateNamespace("prometheus"),
	)

	test.Testenv.Finish(
		envfuncs.DeleteNamespace("slinky"),
		envfuncs.DeleteNamespace("slurm"),
		envfuncs.DeleteNamespace("cert-manager"),
		envfuncs.DeleteNamespace("mariadb"),
		envfuncs.DeleteNamespace("prometheus"),
	)

	os.Exit(test.Testenv.Run(m))
}

// testMainKindCluster runs e2e tests by creating a disposable kind cluster.
func testMainKindCluster(m *testing.M) {
	test.Testenv = env.New()
	kindClusterName := envconf.RandomName("test-e2e", 16)

	operatorName := "ghcr.io/slinkyproject/slurm-operator:" + test.TestUID
	webhookName := "ghcr.io/slinkyproject/slurm-operator-webhook:" + test.TestUID
	if err := test.BuildOperatorImages(operatorName, webhookName); err != nil {
		fmt.Printf("Failed to build images for Slurm-operator: %v", err)
		os.Exit(1)
	}

	test.OperatorImage = operatorName
	test.WebhookImage = webhookName

	buildHelmChart()

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

	test.Testenv.Finish(
		envfuncs.DeleteNamespace("slinky"),
		envfuncs.DeleteNamespace("slurm"),
		envfuncs.DeleteNamespace("cert-manager"),
		envfuncs.DeleteNamespace("mariadb"),
		envfuncs.DeleteNamespace("prometheus"),
		envfuncs.DestroyCluster(kindClusterName),
	)

	os.Exit(test.Testenv.Run(m))
}

func buildHelmChart() {
	slurmOperatorCRDs := action.Package{
		DependencyUpdate: true,
		Destination:      test.Basepath + "helm/slurm-operator/charts",
	}
	if _, err := slurmOperatorCRDs.Run(test.Basepath+"helm/slurm-operator-crds", nil); err != nil {
		fmt.Printf("Failed to build Helm chart for Slurm-operator: %v", err)
		os.Exit(1)
	}
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
			dependencies: []types.Feature{
				installPrometheus(),
			},
		},
		{
			name:    "Install Slurm with accounting",
			install: true,
			test:    true,
			config: test.SlurmInstallationConfig{
				Accounting: true,
			},
			dependencies: []types.Feature{
				installMariadbOperator(),
				applyMariaDBYaml(),
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
			name: "Install Slurm with Pyxis, Login, and Accounting",
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
