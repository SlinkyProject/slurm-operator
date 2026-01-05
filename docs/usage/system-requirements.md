# System Requirements Guide

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [System Requirements Guide](#system-requirements-guide)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Kubernetes](#kubernetes)
    - [Hardware](#hardware)
    - [Storage Class](#storage-class)
  - [Operator](#operator)
    - [Operating System & Architecture](#operating-system--architecture)
    - [Hardware](#hardware-1)
  - [Slurm](#slurm)
    - [Operating System & Architecture](#operating-system--architecture-1)
    - [Hardware](#hardware-2)

<!-- mdformat-toc end -->

## Overview

This guide provides guidance on recommended hardware to run the Slurm Operator
and Slurm clusters on Kubernetes.

## Kubernetes

### Hardware

Generally, your Kubernetes cluster should consist of more than one node where
you have at least one control-plane.

> [!NOTE]
> It is impossible for us to provide a minimum system requirement for your
> workloads.

### Storage Class

It is recommended to have at least one [storage class][storageclass] and a
[default storage class][default-storageclass]. Slurm and other services
installed on your Kubernetes cluster may use
[Persistent Volume Claim (PVC)][persistent-volume] and
[Persistent Volume (PV)][persistent-volume].

## Operator

The operator components consist of:

- `slurm-operator`
- `slurm-operator-webhook`

### Operating System & Architecture

Slinky container images are built on a [distroless] image.

The following machine architectures are supported:

- amd64 (x86_64)
- arm64 (aarch64)

Inspect the [OCI artifacts][oci-slurm-operator] for specific details.

### Hardware

The operator benefits from more cores and memory due to handling requests over
the network and responding. The amount of cores and memory depends on how how
many worker threads were configured and how busy the operator is.

> [!NOTE]
> It is impossible for us to provide a minimum system requirement for your
> workloads. While the operator can run with 1 core and 1GB of memory,
> production usage may find these resources insufficient.

## Slurm

Slurm components consist of:

- `slurmctld`
- `slurmd`
- `slurmdbd`
- `slurmrestd`
- `sackd`

For more information, see the [Slurm docs][slurm-docs].

### Operating System & Architecture

Slurm has broad support for Linux distributions and limited support for FreeBSD
and NetBSD.

> Slurm has been thoroughly tested on most popular Linux distributions using
> arm64 (aarch64), ppc64, and x86_64 architectures. Some features are limited to
> recent releases and newer Linux kernel versions.

See the Slurm [doc][platforms] for details.

The Slurm [container] images built for Slinky only cover a subset of Slurm's
operating system and architecture support.

The following machine architectures are supported:

- amd64 (x86_64)
- arm64 (aarch64)

Inspect the [OCI artifacts][oci-containers] for specific details.

### Hardware

All Slurm daemons benefit from more cores and memory due to handling requests
over the network and responding. The amount of cores and memory depends on how
busy your cluster is. Due to internal data locks, there is a balance to core
counts and single core performance. Some daemons are more sensitive than others.
Some Slurm daemons benefit from fast storage in select areas of the filesystem.
All daemons prefer to not have noisy neighbors, so to speak -- other processes
on the machine cause contention for cores and memory.

Below are notes of interest:

- slurmctld
  - Scheduling benefits greatly from single core performance
  - [StateSaveLocation] benefits greatly from fast storage
- slurmd
  - [SlurmdSpoolDir] benefits greatly from fast storage
  - Depending on the users' jobs, hardware considerations should be made
- slurmdbd
  - Benefits from being co-located on the Database machine, communicating over a
    socket instead of a network.
- slurmrestd
  - Benefits from being co-located on the slurmctld and/or slurmdbd machine,
    communicating over a socket instead of a network.
- sackd
  - Treat like [munged] for the purposes of system requirements
- Database
  - Benefits from fast storage

See the [field notes][slug22-field-notes], slide 17, for notes on system
requirements.

> [!NOTE]
> It is impossible for us to provide a minimum system requirement for your
> workloads. While Slurm daemons can run with 1 core and 1GB of memory,
> production usage may find these resources insufficient.

<!-- Links -->

[container]: https://github.com/SlinkyProject/containers
[default-storageclass]: https://kubernetes.io/docs/concepts/storage/storage-classes/#default-storageclass
[distroless]: https://github.com/GoogleContainerTools/distroless
[munged]: https://dun.github.io/munge/
[oci-containers]: https://github.com/orgs/SlinkyProject/packages?repo_name=containers
[oci-slurm-operator]: https://github.com/orgs/SlinkyProject/packages?repo_name=slurm-operator
[persistent-volume]: https://kubernetes.io/docs/concepts/storage/persistent-volumes/
[platforms]: https://slurm.schedmd.com/platforms.html#os
[slug22-field-notes]: https://slurm.schedmd.com/SLUG22/Field_Notes_6.pdf
[slurm-docs]: ../concepts/slurm.md
[slurmdspooldir]: https://slurm.schedmd.com/slurm.conf.html#OPT_SlurmdSpoolDir
[statesavelocation]: https://slurm.schedmd.com/slurm.conf.html#OPT_StateSaveLocation
[storageclass]: https://kubernetes.io/docs/concepts/storage/storage-classes/
