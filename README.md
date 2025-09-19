# Kubernetes Operator for Slurm Clusters

<div align="center">

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg?style=for-the-badge)](./LICENSES/Apache-2.0.txt)
[![Tag](https://img.shields.io/github/v/tag/SlinkyProject/slurm-operator?style=for-the-badge)](https://github.com/SlinkyProject/slurm-operator/tags/)
[![Go-Version](https://img.shields.io/github/go-mod/go-version/SlinkyProject/slurm-operator?style=for-the-badge)](./go.mod)
[![Last-Commit](https://img.shields.io/github/last-commit/SlinkyProject/slurm-operator?style=for-the-badge)](https://github.com/SlinkyProject/slurm-operator/commits/)

</div>

Run [Slurm] on [Kubernetes], by [SchedMD]. A [Slinky] project.

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Kubernetes Operator for Slurm Clusters](#kubernetes-operator-for-slurm-clusters)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
    - [Slurm Cluster](#slurm-cluster)
  - [Features](#features)
    - [NodeSets](#nodesets)
    - [LoginSets](#loginsets)
    - [Hybrid Support](#hybrid-support)
    - [Slurm](#slurm)
  - [Compatibility](#compatibility)
  - [Quick Start](#quick-start)
  - [Upgrades](#upgrades)
    - [0.X Releases](#0x-releases)
  - [Documentation](#documentation)
  - [Support and Development](#support-and-development)
  - [License](#license)

<!-- mdformat-toc end -->

## Overview

[Slurm] and [Kubernetes] are workload managers originally designed for different
kinds of workloads. In broad strokes: Kubernetes excels at scheduling workloads
that typically run for an indefinite amount of time, with potentially vague
resource requirements, on a single node, with loose policy, but can scale its
resource pool infinitely to meet demand; Slurm excels at quickly scheduling
workloads that run for a finite amount of time, with well defined resource
requirements and topology, on multiple nodes, with strict policy, but its
resource pool is known.

This project enables the best of both workload managers, unified on Kubernetes.
It contains a [Kubernetes] operator to deploy and manage certain components of
[Slurm] clusters. This repository implements [custom-controllers] and
[custom resource definitions (CRDs)][crds] designed for the lifecycle (creation,
upgrade, graceful shutdown) of Slurm clusters.

<img src="./docs/_static/images/architecture-operator.svg" alt="Slurm Operator Architecture" width="100%" height="auto" />

For additional architectural notes, see the [architecture] docs.

### Slurm Cluster

Slurm clusters are very flexible and can be configured in various ways. Our
Slurm helm chart provides a reference implementation that is highly customizable
and tries to expose everything Slurm has to offer.

<img src="./docs/_static/images/architecture-slurm.svg" alt="Slurm Architecture" width="100%" height="auto" />

For additional information about Slurm, see the [slurm][slurm-docs] docs.

## Features

### NodeSets

A set of homogeneous Slurm workers (compute nodes), which are delegated to
execute the Slurm workload.

The operator will take into consideration the running workload among Slurm nodes
as it needs to scale-in, upgrade, or otherwise handle node failures. Slurm nodes
will be marked as [drain][slurm-drain] before their eventual termination pending
scale-in or upgrade.

Slurm node states (e.g. Idle, Allocated, Mixed, Down, Drain, Not Responding,
etc...) are applied to each NodeSet pod via their pod conditions; each NodeSet
pod contains a pod status that reflects their own Slurm node state.

The operator supports NodeSet scale to zero, scaling the resource down to zero
replicas. Hence, any Horizontal Pod Autoscaler (HPA) that also support scale to
zero can be best paired with NodeSets.

NodeSets support optional hostname services, which create a cluster-wide headless
Kubernetes service for all worker pods in the same Slurm cluster. This enables 
hostname-based resolution between login pods and worker pods, enabling direct 
pod-to-pod communication using predictable hostnames (e.g., `cpu-1-0`, `gpu-2-1`). 
The hostname service uses a multi-owner pattern and is automatically managed during 
scaling operations.

### LoginSets

A set of homogeneous login nodes (submit node, jump host) for Slurm, which
manage user identity via SSSD.

The operator supports LoginSet scale to zero, scaling the resource down to zero
replicas. Hence, any Horizontal Pod Autoscaler (HPA) that also support scale to
zero can be best paired with LoginSets.

### Hybrid Support

Sometimes a Slurm cluster has some, but not all, of its components in
Kubernetes. The operator and its CRD are designed support these use cases.

### Slurm

Slurm is a full featured HPC workload manager. To highlight a few features:

- [**Accounting**][slurm-accounting]: collect accounting information for every
  job and job step executed.
- [**Partitions**][slurm-arch]: job queues with sets of resources and
  constraints (e.g. job size limit, job time limit, users permitted).
- [**Reservations**][slurm-reservations]: reserve resources for jobs being
  executed by select users and/or select accounts.
- [**Job Dependencies**][slurm-dependency]: defer the start of jobs until the
  specified dependencies have been satisfied.
- [**Job Containers**][slurm-containers]: jobs which run an unprivileged OCI
  container bundle.
- [**MPI**][slurm-mpi]: launch parallel MPI jobs, supports various MPI
  implementations.
- [**Priority**][slurm-priority]: assigns priorities to jobs upon submission and
  on an ongoing basis (e.g. as they age).
- [**Preemption**][slurm-preempt]: stop one or more low-priority jobs to let a
  high-priority job run.
- [**QoS**][slurm-qos]: sets of policies affecting scheduling priority,
  preemption, and resource limits.
- [**Fairshare**][slurm-fairshare]: distribute resources equitably among users
  and accounts based on historical usage.
- [**Node Health Check**][slurm-healthcheck]: periodically check node health via
  script.

## Compatibility

| Software   |                             Minimum Version                              |
| :--------- | :----------------------------------------------------------------------: |
| Kubernetes | [v1.29](https://kubernetes.io/blog/2023/12/13/kubernetes-v1-29-release/) |
| Slurm      | [25.05](https://www.schedmd.com/slurm-version-25-05-0-is-now-available/) |

## Quick Start

Install the [cert-manager] with its CRDs:

```sh
helm install cert-manager oci://quay.io/jetstack/charts/cert-manager \
  --set 'crds.enabled=true' \
  --namespace cert-manager --create-namespace
```

Install the slurm-operator and its CRDs:

```sh
helm install slurm-operator-crds oci://ghcr.io/slinkyproject/charts/slurm-operator-crds
helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
  --namespace=slinky --create-namespace
```

Install a Slurm cluster:

```sh
helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
  --namespace=slurm --create-namespace
```

For additional instructions, see the [installation] guide.

## Upgrades

### 0.X Releases

Breaking changes may be introduced into newer [CRDs]. To upgrade between these
versions, uninstall all Slinky charts and delete Slinky CRDs, then install the
new release like normal.

```bash
helm --namespace=slurm uninstall slurm
helm --namespace=slinky uninstall slurm-operator
helm --namespace=slinky uninstall slurm-operator-crds
```

If the CRDs were not installed via `slurm-operator-crds` helm chart:

```bash
kubectl delete customresourcedefinitions.apiextensions.k8s.io accountings.slinky.slurm.net
kubectl delete customresourcedefinitions.apiextensions.k8s.io clusters.slinky.slurm.net # defunct
kubectl delete customresourcedefinitions.apiextensions.k8s.io loginsets.slinky.slurm.net
kubectl delete customresourcedefinitions.apiextensions.k8s.io nodesets.slinky.slurm.net
kubectl delete customresourcedefinitions.apiextensions.k8s.io restapis.slinky.slurm.net
kubectl delete customresourcedefinitions.apiextensions.k8s.io tokens.slinky.slurm.net
```

## Documentation

Project documentation is located in the docs directory of this repository.

Slinky documentation can be found [here][slinky-docs].

## Support and Development

Feature requests, code contributions, and bug reports are welcome!

Github/Gitlab submitted issues and PRs/MRs are handled on a best effort basis.

The SchedMD official issue tracker is at <https://support.schedmd.com/>.

To schedule a demo or simply to reach out, please
[contact SchedMD][contact-schedmd].

## License

Copyright (C) SchedMD LLC.

Licensed under the
[Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0) you
may not use project except in compliance with the license.

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.

<!-- links -->

[architecture]: ./docs/architecture.md
[cert-manager]: https://cert-manager.io/docs/installation/helm/
[contact-schedmd]: https://www.schedmd.com/slurm-resources/contact-schedmd/
[crds]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions
[custom-controllers]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers
[installation]: ./docs/installation.md
[kubernetes]: https://kubernetes.io/
[schedmd]: https://schedmd.com/
[slinky]: https://slinky.ai/
[slinky-docs]: https://slinky.schedmd.com/
[slurm]: https://slurm.schedmd.com/overview.html
[slurm-accounting]: https://slurm.schedmd.com/accounting.html
[slurm-arch]: https://slurm.schedmd.com/quickstart.html#arch
[slurm-containers]: https://slurm.schedmd.com/containers.html
[slurm-dependency]: https://slurm.schedmd.com/sbatch.html#OPT_dependency
[slurm-docs]: ./docs/slurm.md
[slurm-drain]: https://slurm.schedmd.com/scontrol.html#OPT_DRAIN
[slurm-fairshare]: https://slurm.schedmd.com/fair_tree.html
[slurm-healthcheck]: https://slurm.schedmd.com/slurm.conf.html#OPT_HealthCheckProgram
[slurm-mpi]: https://slurm.schedmd.com/mpi_guide.html
[slurm-preempt]: https://slurm.schedmd.com/preempt.html
[slurm-priority]: https://slurm.schedmd.com/priority_multifactor.html
[slurm-qos]: https://slurm.schedmd.com/qos.html
[slurm-reservations]: https://slurm.schedmd.com/reservations.html
