# Development

This document aims to provide enough information that you can get started with
development on this project.

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Development](#development)
  - [Table of Contents](#table-of-contents)
  - [Getting Started](#getting-started)
    - [Dependencies](#dependencies)
      - [Pre-Commit](#pre-commit)
      - [Docker](#docker)
      - [Helm](#helm)
      - [Skaffold](#skaffold)
      - [Kubernetes Client](#kubernetes-client)
    - [Running on the Cluster](#running-on-the-cluster)
      - [Automatic](#automatic)
  - [Operator](#operator)
    - [Install CRDs](#install-crds)
    - [Uninstall CRDs](#uninstall-crds)
    - [Modifying the API Definitions](#modifying-the-api-definitions)
    - [Slurm Version Changed](#slurm-version-changed)
    - [Running the operator locally](#running-the-operator-locally)
    - [Slurm Cluster](#slurm-cluster)

<!-- mdformat-toc end -->

## Getting Started

You will need a Kubernetes cluster to run against. You can use [KIND] to get a
local cluster for testing, or run against your choice of remote cluster.

**Note**: Your controller will automatically use the current context in your
kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Dependencies

Install [KIND] and [Golang] binaries for [pre-commit] hooks.

```sh
sudo apt-get install golang
make install
```

#### Pre-Commit

Install [pre-commit] and install the git hooks.

```sh
sudo apt-get install pre-commit
pre-commit install
```

#### Docker

Install [Docker] and configure [rootless Docker][rootless-docker].

After, test that your user account and communicate with docker.

```sh
docker run hello-world
```

#### Helm

Install [Helm].

```sh
sudo snap install helm --classic
```

#### Skaffold

Install [Skaffold].

```sh
curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64 && \
sudo install skaffold /usr/local/bin/
```

If [google-cloud-sdk] is installed, [skaffold] is available as an additional
component.

```sh
sudo apt-get install -y google-cloud-cli-skaffold
```

#### Kubernetes Client

Install [kubectl].

```sh
sudo snap install kubectl --classic
```

If [google-cloud-sdk] is installed, [kubectl] is available as an additional
component.

```sh
sudo apt-get install -y kubectl
```

### Running on the Cluster

For development, all [Helm] deployments use a `values-dev.yaml`. If they do not
exist in your environment yet or you are unsure, safely copy the `values.yaml`
as a base by running:

```sh
make values-dev
```

#### Automatic

You can use [Skaffold] to build and push images, and deploy components using:

```sh
cd helm/slurm-operator/
skaffold run
```

**NOTE**: The `skaffold.yaml` is configured to inject the image and tag into the
`values-dev.yaml` so they are correctly referenced.

## Operator

The slurm operator aims to follow the Kubernetes
[Operator pattern][operator-pattern].

It uses [Controllers][operator-controller], which provide a reconcile function
responsible for synchronizing resources until the desired state is reached on
the cluster.

### Install CRDs

When deploying a [helm] chart with [skaffold] or [helm], the CRDs defined in its
`crds/` directory will be installed if not already present in the cluster.

### Uninstall CRDs

To delete the Operator CRDs from the cluster:

```sh
make uninstall
```

> [!WARNING]
> CRDs do not upgrade! The old ones must be uninstalled first so the new ones
> can be installed. This should only be done in development.

### Modifying the API Definitions

If you are editing the API definitions, generate the manifests such as CRs or
CRDs using:

```sh
make manifests
```

### Slurm Version Changed

If the Slurm version has changed, generate the new OpenAPI spec and its golang
client code using:

```sh
make generate
```

> [!NOTE]
> Update code interacting with the API in accordance with the
> [slurmrestd plugin lifecycle][plugin-lifecycle].

### Running the operator locally

Install the operator's CRDs with `make install`.

Launch the operator via the VSCode debugger using the "Launch Operator" launch
task.

Because the operator will be running outside of Kubernetes and needs to
communicate to the Slurm cluster, set the following options in you Slurm helm
chart's `values.yaml`:

- `debug.enable=true`
- `debug.localOperator=true`

If running on a [Kind] cluster, also set:

- `debug.disableCgroups=true`

If the Slurm [helm] chart is being deployed with [skaffold], run
`skaffold run --port-forward --tail`. It is configured to automatically
[port-forward][skaffold-port-forwarding] the restapi for the local operator to
communicate with the Slurm cluster.

If [skaffold] is not used, manually run
`kubectl port-forward --namespace slurm services/slurm-restapi 6820:6820` for
the local operator to communicate with the Slurm cluster.

After starting the operator, verify it is able to contact the Slurm cluster by
checking that the Cluster CR has been marked ready:

```sh
$ kubectl get --namespace slurm clusters.slinky.slurm.net
NAME     READY   AGE
slurm    true    110s
```

See [skaffold port-forwarding][skaffold-port-forwarding] to learn how [skaffold]
automatically detects which services to forward.

### Slurm Cluster

Get into a Slurm pod that can submit workload.

```bash
kubectl --namespace=slurm exec -it deployments/slurm-login -- bash -l
kubectl --namespace=slurm exec -it statefulsets/slurm-controller -- bash -l
```

```bash
cloud-provider-kind -enable-lb-port-mapping &
SLURM_LOGIN_PORT="$(kubectl --namespace=slurm get services -l app.kubernetes.io/name=login,app.kubernetes.io/instance=slurm -o jsonpath="{.items[0].status.loadBalancer.ingress[0].ports[0].port}")"
SLURM_LOGIN_IP="$(kubectl --namespace=slurm get services -l app.kubernetes.io/name=login,app.kubernetes.io/instance=slurm -o jsonpath="{.items[0].status.loadBalancer.ingress[0].ip}")"
ssh -p "$SLURM_LOGIN_PORT" "${USER}@${SLURM_LOGIN_IP}"
```

<!-- Links -->

[docker]: https://docs.docker.com/engine/install/
[golang]: https://go.dev/
[google-cloud-sdk]: https://cloud.google.com/sdk/docs/install
[helm]: https://helm.sh/
[kind]: https://kind.sigs.k8s.io/
[kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/
[operator-controller]: https://kubernetes.io/docs/concepts/architecture/controller/
[operator-pattern]: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
[plugin-lifecycle]: https://slurm.schedmd.com/rest.html#lifecycle
[pre-commit]: https://pre-commit.com/
[rootless-docker]: https://docs.docker.com/engine/security/rootless/
[skaffold]: https://skaffold.dev/docs/install/
[skaffold-port-forwarding]: https://skaffold.dev/docs/port-forwarding/
