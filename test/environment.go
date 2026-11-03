// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"fmt"
	"os"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const E2EKubeContextEnvironment = "E2E_KUBE_CONTEXT"

// E2EConfig writes a private kubeconfig containing only the explicitly selected
// context. Both the framework client and Helm/kubectl subprocesses must use it.
// The caller owns path and must remove it after the suite finishes.
func E2EConfig(path string) (*envconf.Config, error) {
	kubeContext := os.Getenv(E2EKubeContextEnvironment)
	if kubeContext == "" {
		return nil, fmt.Errorf("%s must name the kubeconfig context of the e2e cluster", E2EKubeContextEnvironment)
	}
	cfg, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	cfg.CurrentContext = kubeContext
	if err := clientcmdapi.MinifyConfig(cfg); err != nil {
		return nil, fmt.Errorf("select kubeconfig context %q: %w", kubeContext, err)
	}
	if err := clientcmdapi.FlattenConfig(cfg); err != nil {
		return nil, fmt.Errorf("flatten kubeconfig context %q: %w", kubeContext, err)
	}
	if err := clientcmd.WriteToFile(*cfg, path); err != nil {
		return nil, fmt.Errorf("write e2e kubeconfig: %w", err)
	}
	return envconf.NewWithKubeConfig(path).WithKubeContext(kubeContext), nil
}
