# Architecture

## Table of Contents

* [Architecture](architecture.md#architecture)
  * [Table of Contents](architecture.md#table-of-contents)
  * [Overview](architecture.md#overview)
  * [Operator](architecture.md#operator)
    * [Required Slurm Functionality](architecture.md#required-slurm-functionality)
      * [Configless](architecture.md#configless)
      * [`auth/slurm`](architecture.md#authslurm)
        * [use\_client\_ids](architecture.md#use_client_ids)
      * [`auth/jwt`](architecture.md#authjwt)
      * [Dynamic Nodes](architecture.md#dynamic-nodes)
        * [Dynamic Topology](architecture.md#dynamic-topology)
        * [Node Features](architecture.md#node-features)
  * [Slurm](architecture.md#slurm)
    * [Hybrid](architecture.md#hybrid)
    * [Autoscale](architecture.md#autoscale)
  * [Directory Map](architecture.md#directory-map)
    * [`api/`](architecture.md#api)
    * [`cmd/`](architecture.md#cmd)
    * [`config/`](architecture.md#config)
    * [`docs/`](architecture.md#docs)
    * [`hack/`](architecture.md#hack)
    * [`helm/`](architecture.md#helm)
    * [`internal/`](architecture.md#internal)
    * [`internal/controller/`](architecture.md#internalcontroller)
    * [`internal/webhook/`](architecture.md#internalwebhook)

## Overview

This document describes the high-level architecture of the Slinky `slurm-operator`.

## Operator

The following diagram illustrates the operator, from a communication perspective.

<img src="../.gitbook/assets/architecture-operator.svg" alt="Slurm Operator Architecture" width="100%">

The `slurm-operator` follows the Kubernetes [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

> Operators are software extensions to Kubernetes that make use of custom resources to manage applications and their components. Operators follow Kubernetes principles, notably the control loop.

The `slurm-operator` has one controller for each Custom Resource Definition (CRD) that it is responsible to manage. Each controller has a control loop where the state of the Custom Resource (CR) is reconciled.

Often, an operator is only concerned about data reported by the Kubernetes API. In our case, we are also concerned about data reported by the Slurm API, which influences how the `slurm-operator` reconciles certain CRs.

### Required Slurm Functionality

The operator makes use of certain Slurm features that help enable containerized clusters. The following are required or assumed by the operator:

* [Configless](architecture.md#configless)
* [auth/slurm](architecture.md#authslurm)
  * [use\_client\_ids](architecture.md#use_client_ids)
* [auth/jwt](architecture.md#authjwt)
* [Dynamic nodes](architecture.md#dynamic-nodes)
  * [Dynamic topology](architecture.md#dynamic-topology)

#### Configless

[Configless](https://slurm.schedmd.com/configless_slurm.html) Slurm allows compute nodes (slurmd) and client commands to pull configuration directly from the slurmctld instead of from pre-distributed local files. Configuration remains centralized on the controllers; only the controllers need the full set of config files.

Typically a non-configless Slurm cluster would use a shared filesystem (e.g. NFS, Lustre) to distribute Slurm configuration files and scripts to each Slurm host. In a containerized environment, that shared filesystem is often absent or undesirable. With configless enabled, each slurmd starts with `--conf-server` (or uses DNS SRV records) to fetch config from slurmctld at startup, and the operator sets `SlurmctldParameters=enable_configless` so the controller serves that config.

Within Kubernetes, the slurmctld pod becomes the source of truth for its cluster configuration, and the controller distributes config updates to nodes. Doing so avoids the desync or drift that can be caused by a shared filesystem or by mounting the same config files into every slurmd pod.

#### `auth/slurm`

Instead of [MUNGE](https://github.com/dun/munge) for user authentication and credentials, Slurm (since 23.11) provides its own [auth/slurm](https://slurm.schedmd.com/authentication.html#slurm) plugin that creates and validates credentials. It uses a shared cryptographic key (e.g. `slurm.key`, or `slurm.jwks` for key rotation) on slurmctld, slurmdbd, and all nodes; every host in the cluster must have that key.

Because `auth/slurm` does not depend on an external authentication service such as MUNGE, no sidecar is required in every pod. That simplifies Slurm daemon pod creation.

**use\_client\_ids**

The [use\_client\_ids](https://slurm.schedmd.com/slurm.conf.html#OPT_use_client_ids) option allows the `auth/slurm` plugin to authenticate users without relying on user information from LDAP or the operating system. With `nss_slurm`, user information can be managed on compute nodes by slurmstepd, so the cluster can operate where only login nodes have access to LDAP or OS user data—for example, containerized worker nodes that do not join the site’s directory services.

Some Slurm configuration options require user and group resolution beyond the credential issued by `auth/slurm`. Those options will not work unless that resolution is enabled (e.g. via `nss_slurm` or another mechanism).

#### `auth/jwt`

Slurm supports [JSON Web Tokens (JWT)](https://slurm.schedmd.com/authentication.html#jwt) as an alternative authentication type (`AuthAltType`), used for client-to-server communication (e.g. slurmrestd and the Slurm REST API). The operator obtains a JWT so it can talk to each Slurm cluster it manages via slurmrestd and make decisions based on the current state of the cluster.

#### Dynamic Nodes

The operator assumes each slurmd container is started as a [dynamic node](https://slurm.schedmd.com/dynamic_nodes.html), so it can register with the controller without pre-defining the node in slurm.conf.

**Dynamic Topology**

The operator ensures that each slurmd pod registers with the topology that matches the Kubernetes node it is scheduled on. It injects topology into the pod (e.g. via `POD_TOPOLOGY`) and, after registration, updates the Slurm node’s topology through the Slurm API. As a result, the Slurm [topology configuration](https://slurm.schedmd.com/topology.yaml.html) does not need to enumerate every node in advance for topology-aware scheduling to work on Kubernetes.

See the [topology usage guide](../usage/topology.md) for more.

**Node Features**

The operator can also propagate Slurm node features from the Kubernetes node a slurmd pod is scheduled on. When the node carries the `features.slinky.slurm.net/spec` annotation, the operator applies its values to the Slurm node's available and active features under a reserved `k8s/` prefix, preserving the NodeSet baseline and any externally-managed features, so jobs can target those nodes with `--constraint=k8s/<feature>`.

See the [node features usage guide](../usage/node-features.md) for more.

## Slurm

The following diagram illustrates a containerized Slurm cluster, from a communication perspective.

<img src="../.gitbook/assets/architecture-slurm.svg" alt="Slurm Cluster Architecture" width="100%">

For additional information about Slurm, see the [slurm](slurm.md) docs.

### Hybrid

The following hybrid diagram is an example. There are many different configurations for a hybrid setup. The core takeaways are: slurmd can be on bare-metal and still be joined to your containerized Slurm cluster; external services that your Slurm cluster needs or wants (e.g. AD/LDAP, NFS, MariaDB) do not have to live in Kubernetes to be functional with your Slurm cluster.

<img src="../.gitbook/assets/architecture-slurm-hybrid.svg" alt="Hybrid Slurm Cluster Architecture" width="100%">

### Autoscale

Kubernetes supports resource autoscaling. In the context of Slurm, autoscaling Slurm workers can be quite useful when your Kubernetes and Slurm clusters have workload fluctuations.

<img src="../.gitbook/assets/architecture-autoscale.svg" alt="Autoscale Architecture" width="100%">

See the [autoscaling](../usage/autoscaling.md) guide for additional information.

## Directory Map

This project follows the conventions of:

* [Golang](https://go.dev/doc/modules/layout)
* [operator-sdk](https://sdk.operatorframework.io/)
* [Kubebuilder](https://book.kubebuilder.io/)

### `api/`

Contains Custom Kubernetes API definitions. These become Custom Resource Definitions (CRDs) and are installed into a Kubernetes cluster.

### `cmd/`

Contains code to be compiled into binary commands.

### `config/`

Contains yaml configuration files used for [kustomize](https://kustomize.io/) deployments.

### `docs/`

Contains project documentation.

### `hack/`

Contains files for development and Kubebuilder. This includes a kind.sh script that can be used to create a kind cluster with all pre-requisites for local testing.

### `helm/`

Contains [helm](https://helm.sh/) deployments, including the configuration files such as values.yaml.

Helm is the recommended method to install this project into your Kubernetes cluster.

### `internal/`

Contains code that is used internally. This code is not externally importable.

### `internal/controller/`

Contains the controllers.

Each controller is named after the Custom Resource Definition (CRD) it manages.

### `internal/webhook/`

Contains the webhooks.

Each webhook is named after the Custom Resource Definition (CRD) it manages.
