# Architecture

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Architecture](#architecture)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Operator](#operator)
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

[autoscaling]: ../usage/autoscaling.md
[golang-layout]: https://go.dev/doc/modules/layout
[helm]: https://helm.sh/
[kubebuilder]: https://book.kubebuilder.io/
[kustomize]: https://kustomize.io/
[operator-pattern]: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
[operator-sdk]: https://sdk.operatorframework.io/
[slurm]: ./slurm.md
