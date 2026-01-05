# Hybrid

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Hybrid](#hybrid)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Slurm](#slurm)
  - [Networking](#networking)
    - [Host Network](#host-network)
    - [Network Peering](#network-peering)
  - [Slurm Configuration](#slurm-configuration)
    - [External Slurmdbd](#external-slurmdbd)
    - [External Slurmctld](#external-slurmctld)
    - [External Slurmd](#external-slurmd)
    - [External Login](#external-login)
    - [External Slurmrestd](#external-slurmrestd)

<!-- mdformat-toc end -->

## Overview

A hybrid cluster is one that combines more than one type of infrastructure
orchestration -- bare-metal, Virtual Machines (VMs), containers (e.g.
Kubernetes, Docker), and cloud infrastrcture (e.g. AWS, GCP, Azure, OpenStack).

Through the slurm-operator and its CRDs, a hybrid Slurm cluster can be expressed
such that some Slurm cluster components live in Kubernetes and other components
live externally.

## Slurm

Slinky currently requires that Slurm uses [configless], [auth/slurm],
[auth/jwt], and [use_client_ids]. This dictates how Slurm clusters can be
defined.

Store `slurm.key` as a secret in Kubernetes.

```sh
kubectl create secret generic external-auth-slurm \
  --namespace=slurm --from-file="slurm.key=/etc/slurm/slurm.key"
```

Store `jwt_hs256.key` as a secret in Kubernetes.

```sh
kubectl create secret generic external-auth-jwths256 \
  --namespace=slurm --from-file="jwt_hs256.key=/etc/slurm/jwt_hs256.key"
```

## Networking

In the context of a hybrid configuration, there are two traffic routes to take
into account: Internal-Internal communication; and External-Internal
communication. Kubernetes Internal-Internal communication typically is pod to
pod traffic, which is a flat network with DNS. External-Internal communication
typically involves external traffic being proxied via NAT to a pod.

Slurm expects a fully connected network with bidirectional communication between
all Slurm daemons and clients This means NAT type networks will generally impede
communication.

Therefore, the network configuration needs to be configured to allow Slurm
components to directly communicate over the network. There are two setups to
choose from, each with their own benefits and drawbacks.

### Host Network

This approach is about avoiding the Kubernetes NAT by having the Slurm pods use
the Kubernetes node host network directly. While it is the simplest methodology,
it does have [security][pod-security-standards] and Slurm configuration
considerations.

Each Slurm pod would be configured as follows.

```yaml
hostNetwork: true
dNSPolicy: ClusterFirstWithHostNet
```

> [!NOTE]
> Controller and Accounting do not support this option due to Slurm
> configuration race with Kubernetes.

> [!WARNING]
> Only one pod with host network enabled can run on a Kubernetes node at a time.
> It will inherit the node's hostname and will run within the host's namespace,
> giving the pod access to the entire network and all ports.

### Network Peering

This approach configures network peering such that internal and external
services can directly communicate. While it is the most complex methodology, it
does not diminish security and minimal Slurm configurations are needed.

This typically involves configuring an advanced [CNI], like [Calico], with
network [peering][bgp] for bidirectional communication across Kubernetes
boundaries.

Generally, no special configuration is required for the Slurm helm chart.

## Slurm Configuration

Slinky currently requires that Slurm use [configless], [auth/slurm], [auth/jwt],
and [use_client_ids]. This dictates how Slurm clusters can be defined.

Copy `slurm.key` as a secret in Kubernetes.

```sh
kubectl create secret generic external-auth-slurm \
  --namespace=slurm --from-file="slurm.key=/etc/slurm/slurm.key"
```

Copy `jwt_hs256.key` as a secret in Kubernetes.

```sh
kubectl create secret generic external-auth-jwths256 \
  --namespace=slurm --from-file="jwt_hs256.key=/etc/slurm/jwt_hs256.key"
```

When configuring the Slurm helm chart, set the Slurm key and JWT key to the
secrets that were copied into Kubernetes otherwise Slurm components will be
unable to authenticate with the rest of the Slurm cluster.

```yaml
slurmKeyRef:
  name: external-auth-slurm
  key: slurm.key
jwtHs256KeyRef:
  name: external-auth-jwths256
  key: jwt_hs256.key
```

> [!WARNING]
> Mixing containerized slurmd with non-containerized slurmd may be problematic
> due to Slurm's assumed homogeneous configuration across all nodes. Notably,
> `cgroup.conf` with `IgnoreSystemd=yes` may not work on both types of nodes.

### External Slurmdbd

When slurmctld is external to Kubernetes, the Slurm helm chart needs to have the
accounting CR configured such that it knows how to communicate with it.

```yaml
accounting:
  external: true
  externalConfig:
    host: $SLURMDBD_HOST
    port: $SLURMDBD_PORT # Default: 6819
```

### External Slurmctld

When slurmctld is external to Kubernetes, the Slurm helm chart needs to have the
controller CR configured such that it knows how to communicate with it.

```yaml
controller:
  external: true
  externalConfig:
    host: $SLURMCTLD_HOST
    port: $SLURMCTLD_PORT # Default: 6817
```

### External Slurmd

When slurmd is external to Kubernetes, the Slurm helm chart only provides
additional workers. The external slurmd must be started with the following
options.

```sh
slurmd --conf-server "${SLURMCTLD_HOST}:${SLURMCTLD_PORT}"
```

### External Login

When login hosts are external to Kubernetes, the Slurm helm chart only provides
additional login pods. The external sackd must be started with the following
options.

```sh
sackd --conf-server "${SLURMCTLD_HOST}:${SLURMCTLD_PORT}"
```

### External Slurmrestd

The Slurm helm chart always provides a slurmrestd pod such that the
slurm-operator can use it to correctly take action on Slurm resources within
kubernetes.

You may still have a slurmrestd that is accessible outside of Kubernetes to
handles requests outside of Kubernetes.

<!-- Links -->

[auth/jwt]: https://slurm.schedmd.com/authentication.html#jwt
[auth/slurm]: https://slurm.schedmd.com/authentication.html#slurm
[bgp]: https://docs.tigera.io/calico/latest/networking/configuring/bgp
[calico]: https://docs.tigera.io/calico/latest/about/
[cni]: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/
[configless]: https://slurm.schedmd.com/configless_slurm.html
[pod-security-standards]: https://kubernetes.io/docs/concepts/security/pod-security-standards/
[use_client_ids]: https://slurm.schedmd.com/slurm.conf.html#OPT_use_client_ids
