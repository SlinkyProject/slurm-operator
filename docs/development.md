# Development

## Local Kind cluster

Create a development cluster and deploy the operator, CRDs, and a Slurm cluster:

```sh
make kind-start
```

Use `make deploy` for a one-time rebuild or `make debug` for Delve-enabled
images. Delete the cluster with:

```sh
make kind-stop
```

The generated `helm/*/values-dev.yaml` files are sparse, untracked overrides.
Add only values needed for development. Skaffold resets each release to the
current chart defaults before applying these overrides. If an older file is a
full copy of `values.yaml`, replace it with `{}` so it does not pin defaults
from an older checkout.

## Scale testing with KWOK

[KWOK] runs pods on fake nodes without starting containers, so a laptop can hold
thousands of NodeSet pods. Create a cluster with the operator and KWOK, then add
100 fake nodes:

```sh
./hack/kind.sh --crds --operator --kwok
./hack/kwok-nodes.sh
```

Each fake node fits 110 pods. For more nodes, run `./hack/kwok-nodes.sh COUNT`
with the total you need.

Apply the test NodeSet, then scale it:

```sh
kubectl apply -f hack/resources/kwok-nodeset.yaml
kubectl scale nodeset/slurm-kwok --replicas=1000
kubectl get nodeset/slurm-kwok --watch
```

The fixture has no RestApi, so the operator skips its Slurm calls and only the
Kubernetes side is exercised. To remove the fake nodes without deleting the
cluster, run:

```sh
kubectl delete nodes --selector=app.kubernetes.io/managed-by=slurm-operator-kwok
```

## Remote cluster

### Update an existing Slinky installation

Install a compatible released Slinky stack first. The workstation running the
deployment needs Make, Skaffold, Helm, Docker, Kubernetes API access, and push
access to a registry that the cluster can pull from.

Select the Kubernetes context and registry, then deploy from the checkout:

```sh
export SKAFFOLD_KUBE_CONTEXT=my-remote-cluster
export SKAFFOLD_DEFAULT_REPO=ghcr.io/my-user
make deploy
```

Skaffold builds and pushes the operator images, then Helm upgrades the CRDs and
operator using the current chart defaults and `values-dev.yaml` overrides.
Existing release values are not retained, so add required cluster-specific
values to `values-dev.yaml` before deploying. It does not upgrade the Slurm
chart. To test changes to that chart, upgrade it explicitly with the
environment's values:

```sh
helm upgrade slurm ./helm/slurm \
  --kube-context my-remote-cluster \
  --namespace slurm \
  --reset-values \
  --values ./helm/slurm/values-dev.yaml \
  --wait
```

### Bootstrap the current Kubernetes context

To install cert-manager, the CRDs, the locally built operator, and Slurm onto an
existing Kubernetes cluster, select its context and provide a registry that the
cluster can pull from:

```sh
kubectl config use-context my-remote-cluster
./hack/kind.sh \
  --existing-cluster \
  --registry=ghcr.io/my-user \
  --core
```

`--registry` sets `SKAFFOLD_DEFAULT_REPO`; setting that environment variable
directly is equivalent. This workflow requires Make, Skaffold, Helm, kubectl,
Go, yq, Docker or Podman, and push access to the registry.

To install only the external prerequisite, cert-manager, run:

```sh
make prereqs
```

The prerequisites-only command requires Helm and kubectl, but does not require
Skaffold or a local container build toolchain.

<!-- Links -->

[kwok]: https://kwok.sigs.k8s.io/
