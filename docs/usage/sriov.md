# Using SR-IOV with Slurm-operator

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Using SR-IOV with Slurm-operator](#using-sr-iov-with-slurm-operator)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Pre-requisites](#pre-requisites)
  - [Deployment Methods](#deployment-methods)
    - [Using NVIDIA Network Operator](#using-nvidia-network-operator)
    - [DRA Driver for SR-IOV Virtual Functions](#dra-driver-for-sr-iov-virtual-functions)

<!-- mdformat-toc end -->

## Overview

Single-root input/output virtualization ([SR-IOV]) is a technology that allows a
physical PCIe device to present itself as multiple discrete devices. SR-IOV
exposes "Virtual Functions" (VFs), which can be seen as additional devices on
the PCI bus. These VFs can be attached to virtual machines and containers,
allowing direct hardware access to network resources. SR-IOV has been shown to
greatly improve network performance on virtualized systems, and is a key
enabling technology for cloud-native HPC.

For more information on SR-IOV's performance implications, see:

- [SR-IOV in High Performance Computing]
- [SR-IOV Support for Virtualization on InfiniBand Clusters: Early Experience]
- [Supporting High Performance Molecular Dynamics in Virtualized Clusters using IOMMU, SR-IOV, and GPUDirect]
- [SR-IOV: The Key Enabling Technology for Fully Virtualized HPC Clusters].

## Pre-requisites

Neither of the deployment methods outlined in this document, nor their
dependencies, have the capability to enable, manage, and create VFs on the
hardware level. Configuration and creation of VFs must be conducted manually,
prior to attempting these methods.

> [!WARNING]
> As VF creation is ephemeral, VFs must be re-created on each system reboot.

For more information on configuring SR-IOV on Intel and Mellanox hardware, see:

- [NVIDIA | SR-IOV]
- [Setting up Virtual Functions]

Before attempting either deployment method below, enable SR-IOV on your
clusters' nodes, and create VFs. When done successfully, Virtual Functions will
be visible in the output of `lspci`.

```console
$ lspci | grep "Virtual Function"
01:00.2 Ethernet controller: Mellanox Technologies MT27710 Family [ConnectX-4 Lx Virtual Function]
01:00.3 Ethernet controller: Mellanox Technologies MT27710 Family [ConnectX-4 Lx Virtual Function]
01:00.4 Ethernet controller: Mellanox Technologies MT27710 Family [ConnectX-4 Lx Virtual Function]
01:00.5 Ethernet controller: Mellanox Technologies MT27710 Family [ConnectX-4 Lx Virtual Function]
01:00.6 Ethernet controller: Mellanox Technologies MT27710 Family [ConnectX-4 Lx Virtual Function]
01:00.7 Ethernet controller: Mellanox Technologies MT27710 Family [ConnectX-4 Lx Virtual Function]
01:01.0 Ethernet controller: Mellanox Technologies MT27710 Family [ConnectX-4 Lx Virtual Function]
01:01.1 Ethernet controller: Mellanox Technologies MT27710 Family [ConnectX-4 Lx Virtual Function]
```

## Deployment Methods

### Using NVIDIA Network Operator

The [NVIDIA Network Operator] provides the simplest deployment path for using
SR-IOV with slurm-operator. During the installation of the NVIDIA Network
Operator, the SR-IOV Network Operator
[can be deployed using a chart parameter][nvidia-sriov-operator].

### DRA Driver for SR-IOV Virtual Functions

[Dynamic Resource Allocation] is a Kubernetes feature that provides a flexible
way to categorize, request, and use devices in a cluster. [dra-driver-sriov]
provides a DRA driver that enables workloads in Kubernetes to request and
utilize SR-IOV VFs through the native resource allocation system.

Slurm-operator has been proven to be functional with the
[SR-IOV DRA Driver][dra-driver-sriov]. Please refer to the [installation guide]
for instructions for the deployment of [dra-driver-sriov]. Please note that the
installation and configuration of these tools is complex and highly
site-specific.

Prior to attempting installation of [dra-driver-sriov], one must:

- Install a compatible CNI meta-plugin ([reference][install-multus])
- Create a SR-IOV CRD for that CNI ([reference][sriov-crd])
- Install [sriov-cni]
- Install [sriov-network-device-plugin]

> [!TIP]
> If a Slinky NodeSet pod with an SR-IOV network interface gets stuck in
> `CrashLoopBackOff` with logs indicating a failure to contact the slurm
> controller, you will need to modify the [sriov-crd] that you created or
> configure the order of routes in your slurm-operator pod. The `routes` or
> `gateway` field of the IPAM section of the SR-IOV CRD has likely caused the
> SR-IOV device to be set as the default route for the `slurm-worker-*` pod,
> which is causing issues resolving the slurm-controller (which should be done
> on the Kubernetes internal network).

<!-- Links -->

[dra-driver-sriov]: https://github.com/k8snetworkplumbingwg/dra-driver-sriov
[dynamic resource allocation]: https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/
[install-multus]: https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin?tab=readme-ov-file#install-one-compatible-cni-meta-plugin
[installation guide]: https://github.com/k8snetworkplumbingwg/dra-driver-sriov?tab=readme-ov-file#deployment
[nvidia network operator]: https://github.com/Mellanox/network-operator
[nvidia | sr-iov]: https://docs.nvidia.com/doca/sdk/sr-iov/index.html
[nvidia-sriov-operator]: https://github.com/Mellanox/network-operator/tree/master/deployment/network-operator#sr-iov-network-operator
[setting up virtual functions]: https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin/blob/master/docs/vf-setup.md
[sr-iov]: https://docs.nvidia.com/doca/archive/2-9-0-cx8/single+root+io+virtualization+(sr-iov)/index.html
[sr-iov in high performance computing]: https://ntrs.nasa.gov/api/citations/20120003714/downloads/20120003714.pdf
[sr-iov support for virtualization on infiniband clusters: early experience]: https://nowlab.cse.ohio-state.edu/static/media/publications/abstract/sriov-ccgrid13.pdf
[sr-iov: the key enabling technology for fully virtualized hpc clusters]: https://www.slideshare.net/slideshow/comet-sriov-sc13boothgkl/29192782#1
[sriov-cni]: https://github.com/k8snetworkplumbingwg/sriov-cni
[sriov-crd]: https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin/blob/master/deployments/sriov-crd.yaml
[sriov-network-device-plugin]: https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin?tab=readme-ov-file#install-sr-iov-network-device-plugin
[supporting high performance molecular dynamics in virtualized clusters using iommu, sr-iov, and gpudirect]: https://dl.acm.org/doi/pdf/10.1145/2817817.2731194
