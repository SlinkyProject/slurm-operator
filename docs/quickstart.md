# QuickStart Guide

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [QuickStart Guide](#quickstart-guide)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Install](#install)
    - [Slurm Operator](#slurm-operator)
    - [Slurm Cluster](#slurm-cluster)
    - [Testing](#testing)

<!-- mdformat-toc end -->

## Overview

This quickstart guide will help you get the slurm-operator running and deploy
Slurm clusters to Kubernetes.

## Install

### Slurm Operator

Download values and install the slurm-operator from OCI package.

```bash
curl -L https://raw.githubusercontent.com/SlinkyProject/slurm-operator/refs/tags/v0.3.0/helm/slurm-operator/values.yaml \
  -o values-operator.yaml
helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
  --values=values-operator.yaml --version=0.3.0 --namespace=slinky --create-namespace --set cert-manager.enabled=true --set kube-prometheus-stack.enabled=true
```

Make sure the cluster deployed successfully with:

```sh
kubectl --namespace=slinky get pods
```

Output should be similar to:

```sh
NAME                                      READY   STATUS    RESTARTS   AGE
slurm-operator-7444c844d5-dpr5h           1/1     Running   0          5m00s
slurm-operator-webhook-6fd8d7857d-zcvqh   1/1     Running   0          5m00s
```

### Slurm Cluster

Download values and install a Slurm cluster from OCI package.

```bash
curl -L https://raw.githubusercontent.com/SlinkyProject/slurm-operator/refs/tags/v0.3.0/helm/slurm/values.yaml \
  -o values-slurm.yaml
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --values=values-slurm.yaml --version=0.3.0 --namespace=slurm --create-namespace
```

Make sure the slurm cluster deployed successfully with:

```sh
kubectl --namespace=slurm get pods
```

Output should be similar to:

```sh
NAME                              READY   STATUS      RESTARTS      AGE
slurm-accounting-0                1/1     Running     0             5m00s
slurm-compute-debug-0             1/1     Running     0             5m00s
slurm-controller-0                2/2     Running     0             5m00s
slurm-exporter-7b44b6d856-d86q5   1/1     Running     0             5m00s
slurm-mariadb-0                   1/1     Running     0             5m00s
slurm-restapi-5f75db85d9-67gpl    1/1     Running     0             5m00s
```

### Testing

SSH through the login service:

```sh
SLURM_LOGIN_IP="$(kubectl get services -n slurm -l app.kubernetes.io/instance=slurm,app.kubernetes.io/name=login -o jsonpath="{.items[0].status.loadBalancer.ingress[0].ip}")"
## Assuming your public SSH key was configured in `login.rootSshAuthorizedKeys[]`.
ssh -p 2222 root@${SLURM_LOGIN_IP}
## Assuming SSSD is configured.
ssh -p 2222 ${USER}@${SLURM_LOGIN_IP}
```

Then, from a login pod, run Slurm commands to quickly test that Slurm is
functioning:

```sh
sinfo
srun hostname
sbatch --wrap="sleep 60"
squeue
sacct
```

See [Slurm Commands][slurm-commands] for more details on how to interact with
Slurm.

<!-- Links -->

[slurm-commands]: https://slurm.schedmd.com/quickstart.html#commands
