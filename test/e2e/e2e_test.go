// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"flag"
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/types"

	"github.com/SlinkyProject/slurm-operator/test"
)

func parseE2EFlags(imageConfig *test.SlurmImageConfig) {
	flag.StringVar(&imageConfig.Repo, "e2e-image-repo", "ghcr.io/slinkyproject", "container registry + repo providing Slurm container images")
	flag.StringVar(&imageConfig.Tag, "e2e-image-tag", "26.05-ubuntu26.04", "image tag to use for Slurm images")
	flag.Parse()
}

// TestMain configures the environment within which all e2e-tests are run
func TestMain(m *testing.M) {
	parseE2EFlags(&test.SlurmImage)

	path := conf.ResolveKubeConfigFile()
	cfg := envconf.NewWithKubeConfig(path)
	test.Testenv = env.NewWithConfig(cfg)
	test.Basepath = test.GetBasePath()

	// launch package tests
	os.Exit(test.Testenv.Run(m))
}

func TestSlurmChart(t *testing.T) {
	tests := []struct {
		name         string
		install      bool
		test         bool
		dependencies []types.Feature
		config       test.SlurmInstallationConfig
	}{
		{
			name: "Validate Slurm-operator deployment",
			dependencies: []types.Feature{
				testCertMgr(),
				testSlurmOperator(),
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
				testMariadbOperator(),
				applyMariaDBYaml(),
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
				testPrometheus(),
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
	}

	for _, tt := range tests {
		steps := getFeaturesFromConfig(tt.install, tt.test, tt.config, tt.dependencies)

		t.Run(tt.name, func(t *testing.T) {
			if len(steps) == 0 {
				t.Skip("scenario is not configured with any E2E features")
			}

			installAttempted := false
			for _, feature := range steps {
				if feature.Name() == "Helm install slurm" {
					installAttempted = true
				}
				_ = test.Testenv.Test(t, feature)
				if t.Failed() {
					test.CaptureFailureDiagnostics(
						t,
						feature.Name(),
						test.SlurmNamespace,
						test.SlinkyNamespace,
					)
					break
				}
			}

			// Keep cleanup separate from the feature loop so a failed feature can
			// be diagnosed before its resources are removed.
			if installAttempted {
				_ = test.Testenv.Test(t, uninstallSlurm())
			}
		})
	}
}
