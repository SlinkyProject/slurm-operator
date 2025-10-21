# Installation Guide

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Installation Guide](#installation-guide)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Slurm Operator And CRDs](#slurm-operator-and-crds)
    - [With CRDs As Subchart](#with-crds-as-subchart)
    - [Without cert-manager](#without-cert-manager)
  - [Slurm Cluster](#slurm-cluster)
    - [With Accounting](#with-accounting)
      - [Mariadb (Community Edition)](#mariadb-community-edition)
    - [With Metrics](#with-metrics)
    - [With Login](#with-login)
      - [With root Authorized Keys](#with-root-authorized-keys)
      - [Testing Slurm](#testing-slurm)

<!-- mdformat-toc end -->

## Overview

Installation instructions for the Slurm Operator on Kubernetes.

## Slurm Operator And CRDs

Install the [cert-manager] with its CRDs, if not already installed:

```sh
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
  --set 'crds.enabled=true' \
  --namespace cert-manager --create-namespace
```

Install the slurm-operator and its CRDs:

```sh
helm install slurm-operator-crds oci://ghcr.io/slinkyproject/charts/slurm-operator-crds
helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
  --namespace=slinky --create-namespace
```

Check if the slurm-operator deployed successfully:

```sh
kubectl --namespace=slinky get pods --selector='app.kubernetes.io/instance=slurm-operator'
```

The output should be similar to:

```sh
NAME                                      READY   STATUS    RESTARTS   AGE
slurm-operator-5d86d75979-6wflf           1/1     Running   0          1m
slurm-operator-webhook-567c84547b-kr7zq   1/1     Running   0          1m
```

### With CRDs As Subchart

If you intend to manage the slurm-operator and the CRDs in the same helm
release, install it with the `--set 'crds.enabled=true'` argument.

```sh
helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
  --set 'crds.enabled=true' \
  --namespace=slinky --create-namespace
```

### Without cert-manager

If the [cert-manager] is not installed, then install the chart with the
`--set 'certManager.enabled=false'` argument, to avoid signing certificates via
cert-manager.

```sh
helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
  --set 'certManager.enabled=false' \
  --namespace=slinky --create-namespace
```

## Slurm Cluster

Install a Slurm cluster via helm chart:

```sh
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --namespace=slurm --create-namespace
```

Check if the Slurm cluster deployed successfully:

```sh
kubectl --namespace=slurm get pods
```

The output should be similar to:

```sh
NAME                                  READY   STATUS    RESTARTS   AGE
slurm-accounting-0                    1/1     Running   0          2m
slurm-controller-0                    3/3     Running   0          2m
slurm-login-slinky-7ff66445b5-wdjkn   1/1     Running   0          2m
slurm-restapi-77b9f969f7-kh4r8        1/1     Running   0          2m
slurm-worker-slinky-0                 2/2     Running   0          2m
```

### With Accounting

You will need to configure Slurm accounting to point at a database. There are
multiple methods to provide a database for Slurm.

Either use:

- the [mariadb-operator]
- the [mysql-operator]
- any Slurm compatible database
  - mysql/mariadb compatible alternatives
  - managed cloud database service

#### Mariadb (Community Edition)

If you intend to enable accounting, install the [mariadb-operator] and its CRDs,
if not already installed:

```sh
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm repo update
helm install mariadb-operator-crds mariadb-operator/mariadb-operator-crds
helm install mariadb-operator mariadb-operator/mariadb-operator \
  --namespace mariadb --create-namespace
```

Create the slurm namespace.

```sh
kubectl create namespace slurm
```

Create a mariadb database via CR.

```sh
kubectl apply -f - <<EOF
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
  namespace: slurm
spec:
  rootPasswordSecretKeyRef:
    name: mariadb-root
    key: password
    generate: true
  username: slurm
  database: slurm_acct_db
  passwordSecretKeyRef:
    name: mariadb-password
    key: password
    generate: true
  storage:
    size: 1Gi
  myCnf: |
    [mariadb]
    bind-address=*
    default_storage_engine=InnoDB
    binlog_format=row
    innodb_autoinc_lock_mode=2
    innodb_buffer_pool_size=4096M
    innodb_lock_wait_timeout=900
    innodb_log_file_size=1024M
    max_allowed_packet=256M
EOF
```

> [!NOTE]
> The mariadb database example above aligns with the Slurm chart's default
> `accounting.storageConfig`. If your actual database configuration is
> different, then you will have to update the `accounting.storageConfig` to work
> with your configuration.

Then install a Slurm cluster via helm chart with the
`--set 'accounting.enabled=true'` argument.

```sh
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --set 'accounting.enabled=true' \
  --namespace=slurm --create-namespace
```

### With Metrics

If you intend to collect metrics, install prometheus and its CRDs, if not
already installed:

```sh
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install prometheus prometheus-community/kube-prometheus-stack \
  --set 'installCRDs=true' \
  --namespace prometheus --create-namespace
```

Then install a Slurm cluster via helm chart with the
`--set 'slurm-exporter.enabled=true'` argument.

```sh
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --set 'slurm-exporter.enabled=true' \
  --namespace=slurm --create-namespace
```

### With Login

You will need to configure the Slurm chart such that the login pods can
communicate with an identity service via [sssd].

> [!WARNING]
> In this example, you will need to supply an `sssd.conf` (at
> `${HOME}/sssd.conf`) that is configured for your environment.

Install a Slurm cluster via helm chart with the
`--set 'loginsets.slinky.enabled=true'` and
`--set-file "loginsets.slinky.sssdConf=${HOME}/sssd.conf"` arguments.

```sh
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --set 'loginsets.slinky.enabled=true' \
  --set-file "loginsets.slinky.sssdConf=${HOME}/sssd.conf" \
  --namespace=slurm --create-namespace
```

#### With root Authorized Keys

> [!NOTE]
> Even if [sssd] is misconfigured, this method can still be used to SSH into the
> pod.

Install a Slurm cluster via helm chart with the
`--set 'loginsets.slinky.enabled=true'` and
`--set-file "loginsets.slinky.rootSshAuthorizedKeys=${HOME}/.ssh/id_ed25519.pub"`
arguments.

```sh
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --set 'loginsets.slinky.enabled=true' \
  --set-file "loginsets.slinky.rootSshAuthorizedKeys=${HOME}/.ssh/id_ed25519.pub" \
  --namespace=slurm --create-namespace
```

#### Testing Slurm

SSH through the login service:

```sh
SLURM_LOGIN_IP="$(kubectl get services -n slurm slurm-login-slinky -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"
SLURM_LOGIN_PORT="$(kubectl get services -n slurm slurm-login-slinky -o jsonpath='{.status.loadBalancer.ingress[0].ports[0].port}')"
## Assuming your public SSH key was configured in `loginsets.slinky.rootSshAuthorizedKeys`.
ssh -p ${SLURM_LOGIN_PORT:-22} root@${SLURM_LOGIN_IP}
## Assuming SSSD was configured correctly.
ssh -p ${SLURM_LOGIN_PORT:-22} ${USER}@${SLURM_LOGIN_IP}
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

[cert-manager]: https://cert-manager.io/docs/installation/helm/
[mariadb-operator]: https://mariadb.com/docs/tools/mariadb-enterprise-operator/installation/helm
[mysql-operator]: https://dev.mysql.com/doc/mysql-operator/en/mysql-operator-installation-helm.html
[slurm-commands]: https://slurm.schedmd.com/quickstart.html#commands
[sssd]: https://sssd.io/
