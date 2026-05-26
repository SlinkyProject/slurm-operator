# Installation Guide

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Installation Guide](#installation-guide)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Slurm Operator And CRDs](#slurm-operator-and-crds)
    - [Namespace-Scoped Watching](#namespace-scoped-watching)
    - [With CRDs As Subchart](#with-crds-as-subchart)
    - [Without cert-manager](#without-cert-manager)
    - [With an Externally-Managed Webhook Certificate](#with-an-externally-managed-webhook-certificate)
  - [Slurm Cluster](#slurm-cluster)
    - [Controller Persistence](#controller-persistence)
    - [With Accounting](#with-accounting)
      - [Mariadb (Community Edition)](#mariadb-community-edition)
    - [With Metrics](#with-metrics)
    - [With Login](#with-login)
      - [With root Authorized Keys](#with-root-authorized-keys)
      - [Testing Slurm](#testing-slurm)
    - [With GPUs](#with-gpus)
    - [With IMEX](#with-imex)
      - [Limitations](#limitations)
      - [Configuration](#configuration)

<!-- mdformat-toc end -->

## Overview

Installation instructions for the Slurm Operator on Kubernetes.

## Slurm Operator And CRDs

Install the [cert-manager] with its CRDs, if not already installed:

```sh
helm install cert-manager oci://quay.io/jetstack/charts/cert-manager \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true
```

Install the slurm-operator and its CRDs:

```sh
helm install slurm-operator-crds oci://ghcr.io/slinkyproject/charts/slurm-operator-crds
helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
  --namespace=slinky --create-namespace
```

Check if the slurm-operator deployed successfully:

```console
$ kubectl --namespace=slinky get pods --selector='app.kubernetes.io/instance=slurm-operator'
NAME                                      READY   STATUS    RESTARTS   AGE
slurm-operator-5d86d75979-6wflf           1/1     Running   0          1m
slurm-operator-webhook-567c84547b-kr7zq   1/1     Running   0          1m
```

### Namespace-Scoped Watching

By default, the operator and webhook watch resources across all namespaces. To
restrict them to specific namespaces, set the `namespaces` value to a
comma-separated list:

```sh
helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
  --set 'operator.namespaces=slurm-system,production' \
  --set 'webhook.namespaces=slurm-system,production' \
  --namespace=slinky --create-namespace
```

> [!NOTE]
> When namespace scoping is enabled, the operator and webhook will only
> reconcile resources in the listed namespaces. Cluster-scoped resources (e.g.
> Nodes) are always watched regardless of this setting.

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

Without cert-manager, the chart generates a self-signed CA and serving
certificate via Helm's `genCA` / `genSignedCert` functions at render time.

### With an Externally-Managed Webhook Certificate

If your organization issues TLS certificates from its own PKI (HashiCorp Vault
PKI, AWS Private CA, an internal CA, [external-secrets], etc.), you can supply
the webhook serving certificate as a pre-existing `kubernetes.io/tls` Secret.
The chart will neither generate a certificate nor render any cert-manager
resources, and rotation becomes the responsibility of your PKI tooling.

Create the Secret in the release namespace before installing the chart. It must
contain `tls.crt`, `tls.key`, and `ca.crt`, and the certificate's SANs must
include both `<webhook-service>.<namespace>` and
`<webhook-service>.<namespace>.svc` (default service name:
`slurm-operator-webhook`).

```sh
kubectl create namespace slinky
kubectl create secret tls slurm-operator-webhook-ca \
  --namespace=slinky \
  --cert=tls.crt --key=tls.key
kubectl patch secret slurm-operator-webhook-ca \
  --namespace=slinky \
  --type=merge \
  -p "{\"data\":{\"ca.crt\":\"$(base64 < ca.crt | tr -d '\n')\"}}"
```

Then install the chart with `certManager.enabled=false` and
`externalCertInjection.enabled=true`:

```sh
helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
  --set 'certManager.enabled=false' \
  --set 'externalCertInjection.enabled=true' \
  --set 'externalCertInjection.secretName=slurm-operator-webhook-ca' \
  --namespace=slinky --create-namespace
```

The chart reads `ca.crt` from the Secret at install/upgrade time via Helm
`lookup` and inlines it into every webhook's `clientConfig.caBundle`. When you
rotate the certificate, run `helm upgrade` to refresh the `caBundle`.

> [!WARNING]
> `certManager.enabled=true` and `externalCertInjection.enabled=true` are
> mutually exclusive (both would manage the same Secret). The chart will fail at
> template time if you set both.

> [!IMPORTANT]
> `helm template` and `helm install --dry-run=client` cannot contact the
> Kubernetes API, so `lookup` returns empty and the chart fails with "Secret …
> was not found" even when the Secret exists. Use
> `helm install --dry-run=server` to exercise the lookup. For GitOps workflows
> that rely on `helm template` for diffs (helm-diff, ArgoCD, Flux), use the
> webhook annotation pass-through described below instead.

> [!NOTE]
> When migrating from `certManager.enabled=true`, give the BYO Secret a name
> distinct from the chart-managed one. Reusing the cert-manager Secret name is
> supported but risky: if the BYO Secret has not yet been created on a fresh
> install, the Pod silently mounts the stale cert-manager Secret. The chart
> cannot detect this; the distinct-name workflow above avoids the race entirely.

If you would rather have cert-manager's [cainjector] populate `caBundle`
automatically from a Secret your PKI keeps refreshed (so `helm upgrade` is not
needed for rotation), combine `externalCertInjection` with the webhook
annotation pass-through. `externalCertInjection` makes the webhook pod mount the
BYO Secret; the annotations make cainjector keep `caBundle` in sync with the
same Secret's `ca.crt`.

> [!IMPORTANT]
> For cainjector to read a `ca.crt` from a `Secret` (the
> `cert-manager.io/inject-ca-from-secret` annotation), the Secret MUST carry the
> annotation `cert-manager.io/allow-direct-injection: "true"`. Without it,
> cainjector silently refuses to inject and the chart-rendered `caBundle`
> becomes the only source of truth. Add the annotation to your BYO Secret
> (either at create time or via `kubectl annotate`) before installing the chart.

```sh
kubectl create namespace slinky
kubectl create secret tls slurm-operator-webhook-byo \
  --namespace=slinky \
  --cert=tls.crt --key=tls.key
kubectl patch secret slurm-operator-webhook-byo \
  --namespace=slinky \
  --type=merge \
  -p "{\"data\":{\"ca.crt\":\"$(base64 < ca.crt | tr -d '\n')\"}}"
kubectl annotate secret slurm-operator-webhook-byo \
  --namespace=slinky \
  cert-manager.io/allow-direct-injection=true

helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
  --set 'certManager.enabled=false' \
  --set 'externalCertInjection.enabled=true' \
  --set 'externalCertInjection.secretName=slurm-operator-webhook-byo' \
  --set 'webhook.validatingAnnotations.cert-manager\.io/inject-ca-from-secret=slinky/slurm-operator-webhook-byo' \
  --set 'webhook.mutatingAnnotations.cert-manager\.io/inject-ca-from-secret=slinky/slurm-operator-webhook-byo' \
  --namespace=slinky --create-namespace
```

This requires cert-manager (specifically `cainjector`) to be running in the
cluster, but the chart still does not manage the certificate itself. The
chart-rendered `caBundle` (read from the Secret via `lookup` at install time)
and the cainjector-written `caBundle` will be identical on install; on rotation,
cainjector keeps `caBundle` correct without a `helm upgrade`.

> [!IMPORTANT]
> Do not omit `externalCertInjection.enabled=true` and rely on the annotations
> alone. Without it, the chart falls back to the self-signed `genCA` mode and
> the webhook pod serves a chart-generated certificate whose CA your PKI Secret
> does not match — the user's PKI is silently ignored regardless of the
> cainjector annotation.

## Slurm Cluster

Install a Slurm cluster via helm chart:

```sh
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --set-json 'nodesets={"slinky":{}}' \
  --set "partitions.all.enabled=true" \
  --namespace=slurm --create-namespace
```

Check if the Slurm cluster deployed successfully:

```console
$ kubectl --namespace=slurm get pods
NAME                                  READY   STATUS    RESTARTS   AGE
slurm-accounting-0                    1/1     Running   0          2m
slurm-controller-0                    3/3     Running   0          2m
slurm-login-slinky-7ff66445b5-wdjkn   1/1     Running   0          2m
slurm-restapi-77b9f969f7-kh4r8        1/1     Running   0          2m
slurm-worker-slinky-0                 2/2     Running   0          2m
```

> [!NOTE]
> The above output is with all Slurm components enabled and configured properly.

### Controller Persistence

By default, the Slurm controller (slurmctld) pod will store its
[state save][statesavelocation] data to a
[Persistent Volume (PV)][persistent-volume]. Its
[Persistent Volume Claim (PVC)][persistent-volume] requests the Kubernetes
[default Storage Class][default-storageclass].

If a default storage class is not defined or a specific storage class is
desired, then you can install Slurm with the
`--set "controller.persistence.storageClassName=$STORAGE_CLASS"` argument, where
`$STORAGE_CLASS` matches an existing storage class.

```sh
kubectl get storageclasses.storage.k8s.io
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --set "controller.persistence.storageClassName=$STORAGE_CLASS" \
  --namespace=slurm --create-namespace
```

> [!NOTE]
> Typically PVs will not be deleted after the PVC is deleted. Therefore, PVs may
> need to be manually deleted when no longer needed.

If Slurm controller (slurmctld) persistence is not desired (typically for
testing), it can be disabled by installing Slurm with the
`--set 'controller.persistence.enabled=false'` argument.

```sh
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --set 'controller.persistence.enabled=false' \
  --namespace=slurm --create-namespace
```

> [!WARNING]
> Without Slurm controller persistence, the state of the Slurm cluster is lost
> between Controller pod restarts. Moreover, these restarts may impact operation
> of the cluster and running workloads. Hence, disabling persistence is **not**
> recommended for production usage.

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
    size: 16Gi
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

Then enable Slurm metrics and the Prometheus service monitor, for metrics
discovery.

```sh
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --set 'controller.metrics.enabled=true' \
  --set 'controller.metrics.serviceMonitor.enabled=true' \
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

### With GPUs

The following describes how to make GPUs present on a Kubernetes cluster
available within Slurm when using Slurm-operator.

The `gres.conf` must have [GRES] defined for each node with GPUs. For dynamic
GRES detection, it is recommended to use [AutoDetect]. The following example
uses dynamic GRES with NVIDIA GPUs.

```yaml
configFiles:
  gres.conf: |
    AutoDetect=nvidia
```

Slurm requires that [GresTypes] contains the "gpu" resource. Slinky sets this by
default, otherwise set the value in `controller.extraConf` or
`controller.extraConfMap`.

```yaml
controller:
  extraConfMap:
    GresTypes: "gpu"
```

NodeSets should request GPUs in accordance with [device plugins][device-plugins]
or [DRA]. In addition, `extraConf` or `extraConfMap` needs to define a [GRES] in
accordance with the GPUs it should be allocated to.

The following is an example is of a `gpu-gb200` NodeSet which has 4 GB200 GPUs.
This example assumes that the [NVIDIA gpu-operator][nvidia-gpu-operator] is
running on the Kubernetes cluster.

```yaml
nodesets:
  gpu-gb200:
    slurmd:
      resources:
        limits:
          nvidia.com/gpu: 4
    extraConfMap:
      Gres: ["gpu:GB200:4"]
```

Within the NodeSet pod, all GPUs on the underlying host are visible in the
output of the `nvidia-smi` command:

```bash
$ kubectl exec -n slurm slurm-controller-0 -- srun bash -c "echo GPU Devices Available on $(hostname):; printf '\n';nvidia-smi"
GPU Devices Available on validation-k8s-headnode:

Mon Apr 20 17:03:28 2026
+-----------------------------------------------------------------------------------------+
| NVIDIA-SMI 590.48.01              Driver Version: 590.48.01      CUDA Version: 13.1     |
+-----------------------------------------+------------------------+----------------------+
| GPU  Name                 Persistence-M | Bus-Id          Disp.A | Volatile Uncorr. ECC |
| Fan  Temp   Perf          Pwr:Usage/Cap |           Memory-Usage | GPU-Util  Compute M. |
|                                         |                        |               MIG M. |
|=========================================+========================+======================|
|   0  NVIDIA GB200                   On  |   00000008:01:00.0 Off |                    0 |
| N/A   37C    P0            166W / 1200W |       0MiB / 189471MiB |      0%      Default |
|                                         |                        |             Disabled |
+-----------------------------------------+------------------------+----------------------+
|   1  NVIDIA GB200                   On  |   00000009:01:00.0 Off |                    0 |
| N/A   37C    P0            157W / 1200W |       0MiB / 189471MiB |      0%      Default |
|                                         |                        |             Disabled |
+-----------------------------------------+------------------------+----------------------+
|   2  NVIDIA GB200                   On  |   00000018:01:00.0 Off |                    0 |
| N/A   36C    P0            150W / 1200W |       0MiB / 189471MiB |      0%      Default |
|                                         |                        |             Disabled |
+-----------------------------------------+------------------------+----------------------+
|   3  NVIDIA GB200                   On  |   00000019:01:00.0 Off |                    0 |
| N/A   36C    P0            172W / 1200W |       0MiB / 189471MiB |      0%      Default |
|                                         |                        |             Disabled |
+-----------------------------------------+------------------------+----------------------+

+-----------------------------------------------------------------------------------------+
| Processes:                                                                              |
|  GPU   GI   CI              PID   Type   Process name                        GPU Memory |
|        ID   ID                                                               Usage      |
|=========================================================================================|
|  No running processes found                                                             |
+-----------------------------------------------------------------------------------------+
```

### With IMEX

NVIDIA GB200 & GB300 NVL72 systems provide the NVIDIA
[Internode Memory Exchange/Management Service (IMEX)][imex] service. IMEX
facilitates shared memory operations across nodes on an NVLink Fabric. IMEX is
crucial to taking full advantage of the NVL72 rackscale systems.

Slurm-operator supports the use of IMEX channels, in order to provide memory
isolation within an IMEX domain. To use IMEX on Slurm-operator, the
[DRA Driver NVIDIA GPU][dra-driver-nvidia-gpu] should be installed in order to
automate the management and allocation of the IMEX daemons. This can be
installed using the [NVIDIA GPU Operator][nvidia-gpu-operator].

Historically, baremetal implementations of IMEX domain management with Slurm
made use of complicated prolog and epilog scripts to standup IMEX channels on
nodes prior to job launch, and to clean them up after job completion. With
Slurm-operator, these scripts should not be used, as they may interfere with the
operations of the [GPU DRA driver][dra-driver-nvidia-gpu].

#### Limitations

As Slurm-operator relies on [DRA] to configure IMEX channels on the underlying
Kubernetes nodes, Slurm-operator does not support per-job configuration of IMEX
channels. If this is a hard requirement for a site, the ComputeDomain that is
claimed by a NodeSet should contain multiple channels, and prolog and epilog
scripts should be used to limit user access to these channels using Linux
permissions or mounts.

#### Configuration

Below is a simple ComputeDomain, which injects all channels into the NodeSet's
`slurmd` pod:

```yaml
---
apiVersion: resource.nvidia.com/v1beta1
kind: ComputeDomain
metadata:
  name: slurm-compute-domain
  namespace: slurm
spec:
  channel:
    allocationMode: All
    resourceClaimTemplate:
      name: slurm-test-compute-domain
  numNodes: 0
```

The [SwitchType] field in [slurm.conf] should be set so that slurmctld can
identify the type of switch that is being used for application communications.
In Slinky clusters, this can be set on Controllers using the `spec.extraConf`
map:

```yaml
controller:
  extraConfMap:
    SwitchType: "switch/nvidia_imex"
    SwitchParameters: ""
```

Resource limits and claims for Slinky NodeSets can be specified in the
`spec.slurmd.resources` field of the CRD or the Slurm Helm chart:

```yaml
nodesets:
  slinky:
    slurmd:
      resources:
        limits:
          nvidia.com/gpu: 4
        claims:
          - name: compute-domain-channel
```

[DRA] ResourceClaims can be specified for Slinky NodeSets in the Helm chart
values:

```yaml
nodesets:
  slinky:
    podSpec:
      resourceClaims:
        - name: compute-domain-channel
          resourceClaimTemplateName: slurm-test-compute-domain
```

Once the Slurm chart has been deployed, a ResourceClaim should be created for
the NodeSet pod:

```bash
$ kubectl get resourceclaim -n slurm
NAME                                                     STATE                AGE
slurm-worker-slinky-jvb8l-compute-domain-channel-4rpsc   allocated,reserved   7m8s
```

A simple job can be launched to confirm successful creation of the IMEX channels
on the NodeSet pod:

```bash
$ kubectl exec -n slurm slurm-controller-0 -- srun bash -c \ "printf '\n\n'; echo === IMEX Channel Test ===; \
 echo Job \$SLURM_JOB_ID Node \$SLURMD_NODENAME; \
echo Channel devices:; \
ls /dev/nvidia-caps-imex-channels/ | grep channel || echo No channels found; \
ls -la /dev/nvidia-caps-imex-channels/channel* 2>/dev/null || echo No channel devices;"


=== IMEX Channel Test ===
Job 74 Node validation-k8s-gpu-02
Channel devices:
channel1
crw-rw-rw- 1 root root 509, 1 Apr 20 17:08 /dev/nvidia-caps-imex-channels/channel1
```

<!-- Links -->

[autodetect]: https://slurm.schedmd.com/gres.conf.html#OPT_AutoDetect
[cainjector]: https://cert-manager.io/docs/concepts/ca-injector/
[cert-manager]: https://cert-manager.io/docs/installation/helm/
[default-storageclass]: https://kubernetes.io/docs/concepts/storage/storage-classes/#default-storageclass
[device-plugins]: https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/#using-device-plugins
[dra]: https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/
[dra-driver-nvidia-gpu]: https://github.com/kubernetes-sigs/dra-driver-nvidia-gpu
[external-secrets]: https://external-secrets.io/
[gres]: https://slurm.schedmd.com/gres.html
[grestypes]: https://slurm.schedmd.com/slurm.conf.html#OPT_GresTypes
[imex]: https://docs.nvidia.com/multi-node-nvlink-systems/imex-guide/overview.html
[mariadb-operator]: https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/helm.md
[mysql-operator]: https://dev.mysql.com/doc/mysql-operator/en/mysql-operator-installation-helm.html
[nvidia-gpu-operator]: https://github.com/NVIDIA/gpu-operator
[persistent-volume]: https://kubernetes.io/docs/concepts/storage/persistent-volumes/
[slurm-commands]: https://slurm.schedmd.com/quickstart.html#commands
[slurm.conf]: https://slurm.schedmd.com/slurm.conf.html
[sssd]: https://sssd.io/
[statesavelocation]: https://slurm.schedmd.com/slurm.conf.html#OPT_StateSaveLocation
[switchtype]: https://slurm.schedmd.com/slurm.conf.html#OPT_SwitchType
