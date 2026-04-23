# Architecture

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Architecture](#architecture)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Operator](#operator)
    - [Required Slurm Functionality](#required-slurm-functionality)
      - [Configless](#configless)
      - [`auth/slurm`](#authslurm)
        - [use_client_ids](#use_client_ids)
      - [`auth/jwt`](#authjwt)
      - [Dynamic Nodes](#dynamic-nodes)
        - [Dynamic Topology](#dynamic-topology)
  - [Slurm](#slurm)
    - [Hybrid](#hybrid)
    - [Autoscale](#autoscale)
  - [Directory Map](#directory-map)
    - [`api/`](#api)
    - [`cmd/`](#cmd)
    - [`config/`](#config)
    - [`docs/`](#docs)
    - [`hack/`](#hack)
    - [`helm/`](#helm)
    - [`internal/`](#internal)
    - [`internal/controller/`](#internalcontroller)
    - [`internal/webhook/`](#internalwebhook)

<!-- mdformat-toc end -->

## Overview

This document describes the high-level architecture of the Slinky
`slurm-operator`.

## Operator

The following diagram illustrates the operator, from a communication
perspective.

<img src="../_static/images/architecture-operator.svg" alt="Slurm Operator Architecture" width="100%" height="auto" />

The `slurm-operator` follows the Kubernetes
[operator pattern][operator-pattern].

> Operators are software extensions to Kubernetes that make use of custom
> resources to manage applications and their components. Operators follow
> Kubernetes principles, notably the control loop.

The `slurm-operator` has one controller for each Custom Resource Definition
(CRD) that it is responsible to manage. Each controller has a control loop where
the state of the Custom Resource (CR) is reconciled.

Often, an operator is only concerned about data reported by the Kubernetes API.
In our case, we are also concerned about data reported by the Slurm API, which
influences how the `slurm-operator` reconciles certain CRs.

### Required Slurm Functionality

The operator makes use of certain Slurm features that help enable containerized
clusters. The following are required or assumed by the operator:

- [Configless](#configless)
- [auth/slurm](#authslurm)
  - [use_client_ids](#use_client_ids)
- [auth/jwt](#authjwt)
- [Dynamic nodes](#dynamic-nodes)
  - [Dynamic topology](#dynamic-topology)

#### Configless

[Configless] Slurm allows compute nodes (slurmd) and client commands to pull
configuration directly from the slurmctld instead of from pre-distributed local
files. Configuration remains centralized on the controllers; only the
controllers need the full set of config files.

Typically a non-configless Slurm cluster would use a shared filesystem (e.g.
NFS, Lustre) to distribute Slurm configuration files and scripts to each Slurm
host. In a containerized environment, that shared filesystem is often absent or
undesirable. With configless enabled, each slurmd starts with `--conf-server`
(or uses DNS SRV records) to fetch config from slurmctld at startup, and the
operator sets `SlurmctldParameters=enable_configless` so the controller serves
that config.

Within Kubernetes, the slurmctld pod becomes the source of truth for its cluster
configuration, and the controller distributes config updates to nodes. Doing so
avoids the desync or drift that can be caused by a shared filesystem or by
mounting the same config files into every slurmd pod.

#### `auth/slurm`

Instead of [MUNGE] for user authentication and credentials, Slurm (since 23.11)
provides its own [auth/slurm] plugin that creates and validates credentials. It
uses a shared cryptographic key (e.g. `slurm.key`, or `slurm.jwks` for key
rotation) on slurmctld, slurmdbd, and all nodes; every host in the cluster must
have that key.

Because `auth/slurm` does not depend on an external authentication service such
as MUNGE, no sidecar is required in every pod. That simplifies Slurm daemon pod
creation.

##### use_client_ids

The [use_client_ids] option allows the `auth/slurm` plugin to authenticate users
without relying on user information from LDAP or the operating system. With
`nss_slurm`, user information can be managed on compute nodes by slurmstepd, so
the cluster can operate where only login nodes have access to LDAP or OS user
data—for example, containerized worker nodes that do not join the site’s
directory services.

Some Slurm configuration options require user and group resolution beyond the
credential issued by `auth/slurm`. Those options will not work unless that
resolution is enabled (e.g. via `nss_slurm` or another mechanism).

#### `auth/jwt`

Slurm supports [JSON Web Tokens (JWT)][auth/jwt] as an alternative
authentication type (`AuthAltType`), used for client-to-server communication
(e.g. slurmrestd and the Slurm REST API). The operator obtains a JWT so it can
talk to each Slurm cluster it manages via slurmrestd and make decisions based on
the current state of the cluster.

#### Dynamic Nodes

The operator assumes each slurmd container is started as a
[dynamic node][dynamic_nodes], so it can register with the controller without
pre-defining the node in slurm.conf.

Before a NodeSet pod exits, its pre-stop hook marks the Slurm node as `DOWN`
rather than removing it from the cluster. This keeps the node entry intact so
the same node name rejoins cleanly when a replacement pod starts, while still
signalling to the scheduler that the node is unavailable. The hook is
best-effort — pods can be evicted, OOM-killed, or forcibly deleted without it
running.

##### Dynamic Topology

The operator ensures that each slurmd pod registers with the topology that
matches the Kubernetes node it is scheduled on. It injects topology into the pod
(e.g. via `POD_TOPOLOGY`) and, after registration, updates the Slurm node’s
topology through the Slurm API. As a result, the Slurm
[topology configuration][topology.yaml] does not need to enumerate every node in
advance for topology-aware scheduling to work on Kubernetes.

See the [topology usage guide][topology] for more.

## Slurm

The following diagram illustrates a containerized Slurm cluster, from a
communication perspective.

<img src="../_static/images/architecture-slurm.svg" alt="Slurm Cluster Architecture" width="100%" height="auto" />

For additional information about Slurm, see the [slurm] docs.

### Hybrid

The following hybrid diagram is an example. There are many different
configurations for a hybrid setup. The core takeaways are: slurmd can be on
bare-metal and still be joined to your containerized Slurm cluster; external
services that your Slurm cluster needs or wants (e.g. AD/LDAP, NFS, MariaDB) do
not have to live in Kubernetes to be functional with your Slurm cluster.

<img src="../_static/images/architecture-slurm-hybrid.svg" alt="Hybrid Slurm Cluster Architecture" width="100%" height="auto" />

### Autoscale

Kubernetes supports resource autoscaling. In the context of Slurm, autoscaling
Slurm workers can be quite useful when your Kubernetes and Slurm clusters have
workload fluctuations.

<img src="../_static/images/architecture-autoscale.svg" alt="Autoscale Architecture" width="100%" height="auto" />

See the [autoscaling] guide for additional information.

## Directory Map

This project follows the conventions of:

- [Golang][golang-layout]
- [operator-sdk]
- [Kubebuilder]

### `api/`

Contains Custom Kubernetes API definitions. These become Custom Resource
Definitions (CRDs) and are installed into a Kubernetes cluster.

### `cmd/`

Contains code to be compiled into binary commands.

### `config/`

Contains yaml configuration files used for [kustomize] deployments.

### `docs/`

Contains project documentation.

### `hack/`

Contains files for development and Kubebuilder. This includes a kind.sh script
that can be used to create a kind cluster with all pre-requisites for local
testing.

### `helm/`

Contains [helm] deployments, including the configuration files such as
values.yaml.

Helm is the recommended method to install this project into your Kubernetes
cluster.

### `internal/`

Contains code that is used internally. This code is not externally importable.

### `internal/controller/`

Contains the controllers.

Each controller is named after the Custom Resource Definition (CRD) it manages.

### `internal/webhook/`

Contains the webhooks.

Each webhook is named after the Custom Resource Definition (CRD) it manages.

<!-- Links -->

[auth/jwt]: https://slurm.schedmd.com/authentication.html#jwt
[auth/slurm]: https://slurm.schedmd.com/authentication.html#slurm
[autoscaling]: ../usage/autoscaling.md
[configless]: https://slurm.schedmd.com/configless_slurm.html
[dynamic_nodes]: https://slurm.schedmd.com/dynamic_nodes.html
[golang-layout]: https://go.dev/doc/modules/layout
[helm]: https://helm.sh/
[kubebuilder]: https://book.kubebuilder.io/
[kustomize]: https://kustomize.io/
[munge]: https://github.com/dun/munge
[operator-pattern]: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
[operator-sdk]: https://sdk.operatorframework.io/
[slurm]: ./slurm.md
[topology]: ../usage/topology.md
[topology.yaml]: https://slurm.schedmd.com/topology.yaml.html
[use_client_ids]: https://slurm.schedmd.com/slurm.conf.html#OPT_use_client_ids
