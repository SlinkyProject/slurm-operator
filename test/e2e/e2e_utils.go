// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/moby/go-archive"

	dockerbuild "github.com/docker/docker/api/types/build"
	dockerclient "github.com/docker/docker/client"

	// mariadb "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	ptr "k8s.io/utils/ptr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

var basepath string

// getBasePath returns the fully qualified path of the slurm-operator repo within the context in which `go test` is called
func getBasePath() string {
	_, b, _, _ := runtime.Caller(0)
	fullpath := filepath.Dir(b)
	path, _ := strings.CutSuffix(fullpath, "test/e2e")

	return path
}

// BuildOperatorImages builds images for Slurm-operator and Slurm-operator-webhook
func BuildOperatorImages(operatorName string, webhookName string) error {
	imageOS := runtime.GOOS
	imageArch := runtime.GOARCH

	imagePlatform := imageOS + "/" + imageArch
	buildArgs := map[string]*string{
		"TARGETOS":      ptr.To(imageOS),
		"TARGETARCH":    ptr.To(imageArch),
		"BUILDPLATFORM": ptr.To(imagePlatform),
	}

	// Build slurm-operator image
	var operatorTags []string
	operatorTags = append(operatorTags, operatorName)
	err := DockerBuild(operatorTags, "manager", "Dockerfile", basepath, buildArgs)
	if err != nil {
		return err
	}

	// Build slurm-operator-webhook image
	var webhookTags []string
	webhookTags = append(webhookTags, webhookName)
	err = DockerBuild(webhookTags, "webhook", "Dockerfile", basepath, buildArgs)
	if err != nil {
		return err
	}

	return nil
}

// DockerBuild builds a Docker image from the provided parameters and pushes it to the local registry
func DockerBuild(imageTags []string, imageTarget string, dockerfile string, dockerfilePath string, buildArgs map[string]*string) error {
	ctx := context.Background()
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err, " :unable to init client")
	}

	tar, err := archive.TarWithOptions(dockerfilePath, &archive.TarOptions{})
	if err != nil {
		return err
	}

	opts := dockerbuild.ImageBuildOptions{
		Context:    tar,
		Dockerfile: dockerfile,
		Remove:     true,
		Target:     imageTarget,
		Tags:       imageTags,
		BuildArgs:  buildArgs,
	}

	imageBuildResponse, err := cli.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}
	defer imageBuildResponse.Body.Close()
	_, err = io.Copy(os.Stdout, imageBuildResponse.Body)
	if err != nil {
		return err
	}

	return nil
}

func CheckControllerHealth(crClient crclient.Client, ctx context.Context, slurmNamespace string, t *testing.T, config *envconf.Config) {
	// Get Controller CR
	controller := &slinkyv1beta1.Controller{}

	controllerKey := crclient.ObjectKey{
		Namespace: slurmNamespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, controllerKey, controller)
	if err != nil {
		t.Fatal("failed to Get() controller using controller-runtime client")
	}

	controllerUID := controller.UID

	// Get Controller StatefulSet using controller CR
	statefulSetKey := controller.Key()
	statefulset := &appsv1.StatefulSet{}
	err = crClient.Get(ctx, statefulSetKey, statefulset)
	if err != nil {
		t.Fatal("failed to Get() statefulset using controller-runtime client")
	}

	// Confirm ownership of controller statefulset
	for _, owner := range statefulset.OwnerReferences {
		if owner.UID != controllerUID {
			t.Fatalf("dubious ownership of statefulset: %v", statefulset)
		}
	}

	// Wait for controller statefulset to become ready
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(statefulset, func(object k8s.Object) int32 {
		return object.(*appsv1.StatefulSet).Status.ReadyReplicas
	}, 1))
	if err != nil {
		t.Fatalf("timed out waiting for StatefulSet %v to reach a ready state", statefulset.Name)
	}
}

func CheckRestAPIHealth(crClient crclient.Client, ctx context.Context, slurmNamespace string, t *testing.T, config *envconf.Config) {
	// Get RestAPI CR
	restapi := &slinkyv1beta1.RestApi{}

	restapiKey := crclient.ObjectKey{
		Namespace: slurmNamespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, restapiKey, restapi)
	if err != nil {
		t.Fatal("failed to Get() restapi using controller-runtime client")
	}

	restapiUID := restapi.UID

	// Get RestAPI Deployment using RestAPI CR
	deploymentKey := restapi.Key()
	deployment := &appsv1.Deployment{}
	err = crClient.Get(ctx, deploymentKey, deployment)
	if err != nil {
		t.Fatal("failed to Get() deployment using controller-runtime client")
	}

	// Confirm ownership of RestAPI deployment
	for _, owner := range deployment.OwnerReferences {
		if owner.UID != restapiUID {
			t.Fatalf("dubious ownership of deployment: %v", deployment)
		}
	}

	// Check whether RestAPI deployment is healthy
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, 1))
	if err != nil {
		t.Fatalf("timed out waiting for Deployment %v to reach a ready state", deployment.Name)
	}
}

func CheckNodeSetHealth(crClient crclient.Client, ctx context.Context, slurmNamespace string, t *testing.T, config *envconf.Config) {
	// Get NodeSet CR
	nodeSetList := slinkyv1beta1.NodeSetList{}
	err := crClient.List(ctx, &nodeSetList)
	if err != nil {
		t.Fatal("failed to List() NodeSets using controller-runtime client")
	}

	// For every NodeSet CR
	for _, nodeSet := range nodeSetList.Items {
		nodesetUID := nodeSet.UID

		// Get all pods
		podList := corev1.PodList{}
		err = crClient.List(ctx, &podList)
		if err != nil {
			t.Fatal("failed to List() Pods using controller-runtime client")
		}

		// Build a list of pods owned by this NodeSet CR
		ownedPods := corev1.PodList{}
		for _, pod := range podList.Items {
			for _, owner := range pod.OwnerReferences {
				if owner.UID == nodesetUID {
					ownedPods.Items = append(ownedPods.Items, pod)
				}
			}
		}

		// Check the health of all pods owned by this NodeSet CR
		for _, pod := range ownedPods.Items {
			// Check whether pod's containers are healthy
			err := wait.For(conditions.New(config.Client().Resources()).ResourceMatch(&pod, func(object k8s.Object) bool {
				return podutils.IsRunning(&pod)
			}))
			if err != nil {
				t.Fatalf("timed out waiting for Pod %v to reach a ready state", pod.Name)
			}
		}
	}
}

func CheckAccountingHealth(crClient crclient.Client, ctx context.Context, slurmNamespace string, t *testing.T, config *envconf.Config) {
	// Get Accounting CR
	accounting := &slinkyv1beta1.Accounting{}

	accountingKey := crclient.ObjectKey{
		Namespace: slurmNamespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, accountingKey, accounting)
	if err != nil {
		t.Fatal("failed to Get() accounting using accounting-runtime client")
	}

	accountingUID := accounting.UID

	// Get Accounting StatefulSet using accounting CR
	statefulSetKey := accounting.Key()
	statefulset := &appsv1.StatefulSet{}
	err = crClient.Get(ctx, statefulSetKey, statefulset)
	if err != nil {
		t.Fatal("failed to Get() statefulset using controller-runtime client")
	}

	// Confirm ownership of controller statefulset
	for _, owner := range statefulset.OwnerReferences {
		if owner.UID != accountingUID {
			t.Fatalf("dubious ownership of statefulset: %v", statefulset)
		}
	}

	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(statefulset, func(object k8s.Object) int32 {
		return object.(*appsv1.StatefulSet).Status.ReadyReplicas
	}, 1))
	if err != nil {
		t.Fatalf("timed out waiting for StatefulSet %v to reach a ready state", statefulset.Name)
	}
}

func CheckLoginSetHealth(crClient crclient.Client, ctx context.Context, slurmNamespace string, t *testing.T, config *envconf.Config) {
	// Get LoginSet  CR
	loginSet := &slinkyv1beta1.LoginSet{}

	loginSetKey := crclient.ObjectKey{
		Namespace: slurmNamespace,
		Name:      "slurm-login-slinky",
	}

	err := crClient.Get(ctx, loginSetKey, loginSet)
	if err != nil {
		t.Fatal("failed to Get() loginSet using controller-runtime client")
	}

	loginSetUID := loginSet.UID

	// Get loginSet Deployment using loginSet CR
	deploymentKey := loginSet.Key()
	deployment := &appsv1.Deployment{}
	err = crClient.Get(ctx, deploymentKey, deployment)
	if err != nil {
		t.Fatal("failed to Get() deployment using controller-runtime client")
	}

	// Confirm ownership of loginSet deployment
	for _, owner := range deployment.OwnerReferences {
		if owner.UID != loginSetUID {
			t.Fatalf("dubious ownership of deployment: %v", deployment)
		}
	}

	// Check whether loginSet deployment is healthy
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, 1))
	if err != nil {
		t.Fatalf("timed out waiting for Deployment %v to reach a ready state", deployment.Name)
	}
}

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

			CheckControllerHealth(crClient, ctx, slurmNamespace, t, config)
			CheckRestAPIHealth(crClient, ctx, slurmNamespace, t, config)
			CheckNodeSetHealth(crClient, ctx, slurmNamespace, t, config)

			if withAccounting {
				CheckAccountingHealth(crClient, ctx, slurmNamespace, t, config)
			}

			if withLogin {
				CheckLoginSetHealth(crClient, ctx, slurmNamespace, t, config)
			}

			return ctx
		}).Feature()
}

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

func scontrolPing(slurmNamespace string) types.Feature {
	return features.New("scontrol ping succeeds").
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
}

func srun(slurmNamespace string) types.Feature {
	return features.New("srun functions").
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
}

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
