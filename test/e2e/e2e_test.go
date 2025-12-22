// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/support/kind"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	"helm.sh/helm/v3/pkg/action"

	// mariadb "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	)

	// Use pre-defined environment funcs to teardown kind cluster after tests
	testenv.Finish(
		envfuncs.DeleteNamespace("slinky"),
		envfuncs.DeleteNamespace("slurm"),
		envfuncs.DeleteNamespace("cert-manager"),
		envfuncs.DeleteNamespace("mariadb"),
		envfuncs.DestroyCluster(kindClusterName),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}

func TestHelmInstallation(t *testing.T) {
	const slinkyNamespace = "slinky"
	const slurmNamespace = "slurm"

	installCertMgr := features.New("Helm install cert-manager").
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

	installSlurmOperatorCRDS := features.New("Helm install slurm-operator-crds").
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

	installSlurmOperator := features.New("Helm install slurm-operator").
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

	installSlurm := features.New("Helm install slurm").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			manager := helm.New(config.KubeconfigFile())

			setValuesFile := fmt.Sprintf("--values %s/helm/slurm/values.yaml", basepath)

			err := manager.RunInstall(
				helm.WithName("slurm"),
				helm.WithNamespace(slurmNamespace),
				helm.WithChart(basepath+"helm/slurm"),
				helm.WithArgs(setValuesFile),
				helm.WithWait(),
				helm.WithTimeout("10m"))

			if err != nil {
				t.Fatal("failed to invoke helm install operation due to an error", err)
			}
			return ctx
		}).
		Assess("Slurm Cluster Is Running Successfully", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			// Set up controller-runtime client
			var scheme = runtime.NewScheme()
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

			CheckControllerHealth(crClient, ctx, slurmNamespace, t, config)
			CheckRestAPIHealth(crClient, ctx, slurmNamespace, t, config)
			CheckNodeSetHealth(crClient, ctx, slurmNamespace, t, config)

			return ctx
		}).Feature()
	scontrolPing := features.New("scontrol ping succeeds").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			for retries := range 4 {

				command := "kubectl"
				args := []string{"exec", "-n", slurmNamespace, "slurm-controller-0", "--", "scontrol", "ping"}
				cmd := exec.Command(command, args...)

				_, err := cmd.Output()
				if err == nil {
					break
				}

				if retries == 3 {
					t.Fatalf("failed running '%v %v': %v", command, args, err)
					break
				}

				time.Sleep(30 * time.Second)
			}

			return ctx
		}).Feature()
	srun := features.New("srun functions").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			for retries := range 4 {

				cleanup_command := "kubectl"
				cleanup_args := []string{"exec", "-n", slurmNamespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"}
				cleanup_cmd := exec.Command(cleanup_command, cleanup_args...)

				_, _ = cleanup_cmd.Output() //nolint:errcheck

				command := "kubectl"
				args := []string{"exec", "-n", slurmNamespace, "slurm-controller-0", "--", "srun", "hostname"}
				cmd := exec.Command(command, args...)

				_, err := cmd.Output()
				if err == nil {
					break
				}

				if retries == 3 {
					t.Fatalf("failed running '%v %v': %v", command, args, err)
					break
				}

				time.Sleep(30 * time.Second)
			}

			return ctx
		}).Feature()

	uninstallSlurm := features.New("Helm uninstall slurm").
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

	uninstallSlurmOperator := features.New("Helm uninstall slurm-operator").
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

	uninstallSlurmOperatorCRDs := features.New("Helm uninstall slurm-operator-crds").
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

	uninstallCertMgr := features.New("Helm uninstall cert-manager").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			manager := helm.New(config.KubeconfigFile())

			err := manager.RunUninstall(
				helm.WithName("cert-manager"),
				helm.WithWait(),
				helm.WithNamespace("cert-manager"),
				helm.WithTimeout("5m"),
			)

			if err != nil {
				t.Fatal("failed to invoke helm uninstall cert-manager due to an error", err)
			}
			return ctx
		}).Feature()

	_ = testenv.Test(
		t,
		installCertMgr,
		installSlurmOperatorCRDS,
		installSlurmOperator,
		installSlurm,
		scontrolPing,
		srun,
		uninstallSlurm,
		uninstallSlurmOperator,
		uninstallSlurmOperatorCRDs,
		uninstallCertMgr)
}

func TestHelmInstallationWithAccounting(t *testing.T) {
	const slurmNamespace = "slurm"
	const slinkyNamespace = "slinky"

	installCertMgr := features.New("Helm install cert-manager").
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

	installSlurmOperatorCRDS := features.New("Helm install slurm-operator-crds").
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

	installSlurmOperator := features.New("Helm install slurm-operator").
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

	installMariadbOperator := features.New("Helm install mariadb-operator").
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

	applyMariaDBYaml := features.New("Create MariaDB instance for Slurm").
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
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mariadb-0",
					Namespace: slurmNamespace,
				},
			}
			err := wait.For(conditions.New(config.Client().Resources()).PodRunning(pod))
			if err != nil {
				t.Fatal("failed waiting for the mariadb-0 pod to reach a ready state")
			}
			return ctx
		}).Feature()
	installSlurm := NewSlurmInstall(t, slurmNamespace, true)
	scontrolPing := features.New("scontrol ping succeeds").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			for retries := range 4 {

				command := "kubectl"
				args := []string{"exec", "-n", slurmNamespace, "slurm-controller-0", "--", "scontrol", "ping"}
				cmd := exec.Command(command, args...)

				_, err := cmd.Output()
				if err == nil {
					break
				}

				if retries == 3 {
					t.Fatalf("failed running '%v %v': %v", command, args, err)
					break
				}

				time.Sleep(30 * time.Second)
			}

			return ctx
		}).Feature()
	srun := features.New("srun functions").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			for retries := range 4 {

				cleanup_command := "kubectl"
				cleanup_args := []string{"exec", "-n", slurmNamespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"}
				cleanup_cmd := exec.Command(cleanup_command, cleanup_args...)

				_, _ = cleanup_cmd.Output() //nolint:errcheck

				command := "kubectl"
				args := []string{"exec", "-n", slurmNamespace, "slurm-controller-0", "--", "srun", "hostname"}
				cmd := exec.Command(command, args...)

				_, err := cmd.Output()
				if err == nil {
					break
				}

				if retries == 3 {
					t.Fatalf("failed running '%v %v': %v", command, args, err)
					break
				}

				time.Sleep(30 * time.Second)
			}

			return ctx
		}).Feature()

	_ = testenv.Test(t,
		installCertMgr,
		installSlurmOperatorCRDS,
		installSlurmOperator,
		installMariadbOperator,
		applyMariaDBYaml,
		installSlurm,
		scontrolPing,
		srun)
}
