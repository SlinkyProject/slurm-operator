// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

// getBasePath returns the fully qualified path of the slurm-operator repo within the context in which `go test` is called
func GetBasePath() string {
	_, b, _, _ := runtime.Caller(0)
	fullpath := filepath.Dir(b)
	path, _ := strings.CutSuffix(fullpath, "test")

	return path
}

// CheckDeploymentStatus waits for the specified deployment to have the desired number of ReadyReplicas, then returns
func CheckDeploymentStatus(ctx context.Context, t *testing.T, config *envconf.Config, deploymentName string, deploymentNamespace string) context.Context {
	t.Helper()

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: deploymentNamespace,
		},
	}
	err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, 1))
	require.NoError(
		t,
		err,
		"failed waiting for deployment %s/%s to reach one ready replica; observed status: %s",
		deploymentNamespace,
		deploymentName,
		StatusJSON(deployment.Status),
	)

	return ctx
}

// StatusJSON renders Kubernetes status structs compactly for failure messages.
func StatusJSON(status any) string {
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Sprintf("<failed to marshal status: %v>", err)
	}
	return string(data)
}

// DoUninstallHelmChart uses the Helm API to uninstall the specified Helm chart in the specified namespace
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

// DoDeleteNamespace deletes the specified namespace and waits for its removal.
func DoDeleteNamespace(ctx context.Context, t *testing.T, config *envconf.Config, namespace string) context.Context {
	namespaceObject := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}
	resources := config.Client().Resources()
	err := resources.Delete(ctx, namespaceObject)
	if apierrors.IsNotFound(err) {
		return ctx
	}
	require.NoError(t, err, "failed to delete namespace %s", namespace)

	err = wait.For(
		conditions.New(resources).ResourceDeleted(namespaceObject),
		wait.WithContext(ctx),
		wait.WithTimeout(5*time.Minute),
		wait.WithInterval(2*time.Second),
	)
	require.NoError(t, err, "timed out waiting for namespace %s to be deleted", namespace)

	return ctx
}
