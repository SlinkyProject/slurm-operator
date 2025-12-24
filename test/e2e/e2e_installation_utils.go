// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

// Dependency Installation

func installCertMgr() types.Feature {
	return features.New("Helm install cert-manager").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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
		}).
		Assess("cert-manager Deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cert-manager",
					Namespace: "cert-manager",
				},
			}
			err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
				return object.(*appsv1.Deployment).Status.ReadyReplicas
			}, 1))
			if err != nil {
				t.Fatal("failed waiting for the cert-manager deployment to reach a ready state")
			}
			return ctx
		}).
		Assess("cert-manager-cainjector Deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cert-manager-cainjector",
					Namespace: "cert-manager",
				},
			}
			err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
				return object.(*appsv1.Deployment).Status.ReadyReplicas
			}, 1))
			if err != nil {
				t.Fatal("failed waiting for the cert-manager-cainjector deployment to reach a ready state")
			}
			return ctx
		}).
		Assess("cert-manager-webhook deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cert-manager-webhook",
					Namespace: "cert-manager",
				},
			}
			err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
				return object.(*appsv1.Deployment).Status.ReadyReplicas
			}, 1))
			if err != nil {
				t.Fatal("failed waiting for the cert-manager-webhook deployment to reach a ready state")
			}
			return ctx
		}).Feature()
}

func installMariadbOperator() types.Feature {
	return features.New("Helm install mariadb-operator").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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
		}).
		Assess("mariadb-operator deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-operator",
					Namespace: "mariadb",
				},
			}
			err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
				return object.(*appsv1.Deployment).Status.ReadyReplicas
			}, 1))
			if err != nil {
				t.Fatal("failed waiting for the mariadb-operator deployment to reach a ready state")
			}
			return ctx
		}).
		Assess("mariadb-operator-cert-controller deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-operator-cert-controller",
					Namespace: "mariadb",
				},
			}
			err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
				return object.(*appsv1.Deployment).Status.ReadyReplicas
			}, 1))
			if err != nil {
				t.Fatal("failed waiting for the mariadb-operator-cert-controller deployment to reach a ready state")
			}
			return ctx
		}).
		Assess("mariadb-operator-webhook deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-operator-webhook",
					Namespace: "mariadb",
				},
			}
			err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
				return object.(*appsv1.Deployment).Status.ReadyReplicas
			}, 1))
			if err != nil {
				t.Fatal("failed waiting for the mariadb-operator-webhook deployment to reach a ready state")
			}
			return ctx
		}).Feature()
}

func applyMariaDBYaml(slurmNamespace string) types.Feature {
	return features.New("Create MariaDB instance for Slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			path := basepath + "/hack/resources/mariadb.yaml"

			cmd := exec.Command("kubectl", "apply", "-f", path)
			_, err := cmd.Output()
			if err != nil {
				t.Fatalf("failed running 'kubectl apply -f %s /hack/resources/mariadb.yaml: %v", basepath, err)
			}

			return ctx
		}).
		Assess("Pod mariadb-0 running successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			var scheme = k8sruntime.NewScheme()
			err := mariadbv1alpha1.AddToScheme(scheme)
			if err != nil {
				t.Fatalf("failed adding api mariadb")
			}

			err = appsv1.AddToScheme(scheme)
			if err != nil {
				t.Fatalf("failed adding api appsv1 to scheme: %v", err)
			}

			crClient, err := klient.NewControllerRuntimeClient(config.Client().RESTConfig(), scheme)
			if err != nil {
				t.Fatalf("failed creating new controller-runtime client: %v", err)
			}

			checkMariaDBHealth(crClient, ctx, slurmNamespace, t, config)

			return ctx
		}).Feature()
}

func installPrometheus() types.Feature {
	return features.New("Helm install prometheus").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
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
		}).
		Assess("prometheus deployment Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prometheus-kube-prometheus-operator",
					Namespace: "prometheus",
				},
			}
			err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
				return object.(*appsv1.Deployment).Status.ReadyReplicas
			}, 1))
			if err != nil {
				t.Fatal("failed waiting for the prometheus deployment to reach a ready state")
			}
			return ctx
		}).Feature()
}

// Slinky Components Installation

func installSlurm(slurmNamespace string, withAccounting bool, withLogin bool, withMetrics bool) types.Feature {
	return features.New("Helm install slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			manager := helm.New(config.KubeconfigFile())

			setValuesFile := fmt.Sprintf("--values %s/helm/slurm/values.yaml", basepath)

			var err error

			opts := []helm.Option{}
			opts = append(
				opts,
				helm.WithName("slurm"),
				helm.WithNamespace(slurmNamespace),
				helm.WithChart(basepath+"helm/slurm"),
				helm.WithArgs(setValuesFile),
				helm.WithWait(),
				helm.WithTimeout("10m"),
			)

			if withAccounting {
				opts = append(opts, helm.WithArgs("--set 'accounting.enabled=true'"))
			}

			if withLogin {
				opts = append(opts, helm.WithArgs("--set 'loginsets.slinky.enabled=true'"))
			}

			if withMetrics {
				opts = append(opts, helm.WithArgs("--set 'controller.metrics.enabled=true'"))
				opts = append(opts, helm.WithArgs("--set 'controller.metrics.serviceMonitor.enabled=true'"))
			}

			err = manager.RunInstall(opts...)

			if err != nil {
				t.Fatal("failed to invoke helm install operation due to an error", err)
			}
			return ctx
		}).
		Assess("Slurm Cluster Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			// Set up controller-runtime client
			var scheme = k8sruntime.NewScheme()
			err := slinkyv1beta1.AddToScheme(scheme)
			if err != nil {
				t.Fatalf("failed adding api slinkyv1beta1 to scheme: %v", err)
			}
			err = appsv1.AddToScheme(scheme)
			if err != nil {
				t.Fatalf("failed adding api appsv1 to scheme: %v", err)
			}
			err = corev1.AddToScheme(scheme)
			if err != nil {
				t.Fatalf("failed adding api corev1 to scheme: %v", err)
			}

			crClient, err := klient.NewControllerRuntimeClient(config.Client().RESTConfig(), scheme)
			if err != nil {
				t.Fatalf("failed creating new controller-runtime client: %v", err)
			}

			checkControllerHealth(crClient, ctx, slurmNamespace, t, config)
			checkRestAPIHealth(crClient, ctx, slurmNamespace, t, config)
			checkNodeSetHealth(crClient, ctx, slurmNamespace, t, config)

			if withAccounting {
				checkAccountingHealth(crClient, ctx, slurmNamespace, t, config)
			}

			if withLogin {
				checkLoginSetHealth(crClient, ctx, slurmNamespace, t, config)
			}

			return ctx
		}).Feature()
}

func installSlurmOperatorCRDS(slinkyNamespace string) types.Feature {
	return features.New("Helm install slurm-operator-crds").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			manager := helm.New(config.KubeconfigFile())

			err := manager.RunInstall(
				helm.WithName("slurm-operator-crds"),
				helm.WithNamespace(slinkyNamespace),
				helm.WithChart(basepath+"helm/slurm-operator-crds"),
				helm.WithWait(),
				helm.WithTimeout("10m"))
			if err != nil {
				t.Fatal("failed to invoke helm install slurm-operator-crds due to an error", err)
			}
			return ctx
		}).Feature()
}

func installSlurmOperator(testUID string, slinkyNamespace string) types.Feature {
	return features.New("Helm install slurm-operator").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			manager := helm.New(config.KubeconfigFile())

			setOperatorImage := fmt.Sprintf("--set operator.image.tag=%s", testUID)
			setWebhookImage := fmt.Sprintf("--set webhook.image.tag=%s", testUID)

			err := manager.RunInstall(
				helm.WithName("slurm-operator"),
				helm.WithNamespace(slinkyNamespace),
				helm.WithChart(basepath+"helm/slurm-operator"),
				helm.WithWait(),
				helm.WithTimeout("10m"),
				helm.WithArgs(setOperatorImage),
				helm.WithArgs(setWebhookImage))
			if err != nil {
				t.Fatal("failed to invoke helm install slurm-operator due to an error", err)
			}
			return ctx
		}).
		Assess("Deployment slurm-operator running successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slurm-operator",
					Namespace: slinkyNamespace,
				},
				Spec: appsv1.DeploymentSpec{},
			}
			err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
				return object.(*appsv1.Deployment).Status.ReadyReplicas
			}, 1))
			if err != nil {
				t.Fatal("failed waiting for the slurm-operator deployment to reach a ready state")
			}
			return ctx
		}).
		Assess("Deployment slurm-operator-webhook running successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slurm-operator",
					Namespace: slinkyNamespace,
				},
				Spec: appsv1.DeploymentSpec{},
			}
			err := wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
				return object.(*appsv1.Deployment).Status.ReadyReplicas
			}, 1))
			if err != nil {
				t.Fatal("failed waiting for the slurm-operator-webhook deployment to reach a ready state")
			}
			return ctx
		}).Feature()
}

// Uninstall Slurm Components

func uninstallSlurm(slurmNamespace string) types.Feature {
	return features.New("Helm uninstall slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			manager := helm.New(config.KubeconfigFile())

			err := manager.RunUninstall(
				helm.WithName("slurm"),
				helm.WithNamespace(slurmNamespace),
				helm.WithWait(),
				helm.WithTimeout("5m"),
			)

			if err != nil {
				t.Fatal("failed to invoke helm uninstall slurm due to an error", err)
			}
			return ctx
		}).Feature()
}

func uninstallSlurmOperator(slinkyNamespace string) types.Feature {
	return features.New("Helm uninstall slurm-operator").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			manager := helm.New(config.KubeconfigFile())

			err := manager.RunUninstall(
				helm.WithName("slurm-operator"),
				helm.WithNamespace(slinkyNamespace),
				helm.WithWait(),
				helm.WithTimeout("5m"),
			)

			if err != nil {
				t.Fatal("failed to invoke helm uninstall slurm-operator due to an error", err)
			}
			return ctx
		}).Feature()
}

func uninstallSlurmOperatorCRDs(slinkyNamespace string) types.Feature {
	return features.New("Helm uninstall slurm-operator-crds").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			manager := helm.New(config.KubeconfigFile())

			err := manager.RunUninstall(
				helm.WithName("slurm-operator-crds"),
				helm.WithWait(),
				helm.WithNamespace(slinkyNamespace),
				helm.WithTimeout("5m"),
			)

			if err != nil {
				t.Fatal("failed to invoke helm uninstall slurm-operator-crds due to an error", err)
			}
			return ctx
		}).Feature()
}
