// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/test"
)

func getFeaturesFromConfig(install bool, runTests bool, config test.SlurmInstallationConfig, beforeSteps []types.Feature) []types.Feature {
	steps := beforeSteps

	if install {
		steps = append(steps, installSlurm(config))
	}
	if runTests {

		steps = append(steps, testSlurmController())
		steps = append(steps, testSlurmRestAPI(config.Accounting))
		steps = append(steps, testSlurmNodeSet())

		if config == (test.SlurmInstallationConfig{}) {
			steps = append(steps, testSlurmJWTKeyRotation())
		}

		if config.Accounting {
			steps = append(steps, testSlurmAccounting())
		}
	}

	return steps
}

func GetControllerRuntimeClient(config *envconf.Config) (crclient.Client, error) {
	var scheme = k8sruntime.NewScheme()
	err := slinkyv1beta1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = appsv1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = corev1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	err = mariadbv1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	return klient.NewControllerRuntimeClient(config.Client().RESTConfig(), scheme)
}
