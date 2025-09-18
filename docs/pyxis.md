# Pyxis Guide

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Pyxis Guide](#pyxis-guide)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Configure](#configure)
  - [Test](#test)

<!-- mdformat-toc end -->

## Overview

This guide tells how to configure your Slurm cluster to use [pyxis] (and
[enroot]), a Slurm [SPANK] plugin for containerized jobs with Nvidia GPU
support.

## Configure

Configure `plugstack.conf` to include the pyxis configuration.

> [!WARNING]
> In `plugstack.conf`, you must use glob syntax to avoid slurmctld failure while
> trying to resolve the paths in the includes. Only the login and slurmd pods
> should actually have the pyxis libraries installed.

```yaml
configFiles:
  plugstack.conf: |
    include /usr/share/pyxis/*
  ...
```

Configure one or more NodeSets and the login pods to use a pyxis OCI image.

```yaml
loginsets:
  - name: pyxis
    image:
      repository: ghcr.io/slinkyproject/login-pyxis
    ...
nodesets:
  - name: pyxis
    image:
      repository: ghcr.io/slinkyproject/slurmd-pyxis
    ...
```

To make enroot activity in the login container permissible, it requires
`securityContext.privileged=true`.

```yaml
loginsets:
  - name: pyxis
    image:
      repository: ghcr.io/slinkyproject/login-pyxis
    securityContext:
      privileged: true
    ...
```

## Test

Submit a job to a Slurm node.

```bash
$ srun --partition=pyxis grep PRETTY /etc/os-release
PRETTY_NAME="Ubuntu 24.04.2 LTS"
```

Submit a job to a Slurm node with pyxis and it will launch in its requested
container.

```bash
$ srun --partition=pyxis --container-image=alpine:latest grep PRETTY /etc/os-release
pyxis: importing docker image: alpine:latest
pyxis: imported docker image: alpine:latest
PRETTY_NAME="Alpine Linux v3.21"
```

> [!WARNING]
> SPANK plugins will only work on specific Slurm node that have them and is
> configured to use them. It is best to constrain where jobs run with
> `--partition=<partition>`, `--batch=<features>`, and/or
> `--constraint=<features>` to ensure a compatible computing environment.

If the login container has `securityContext.privileged=true`, enroot activity is
permissible. You can test the functionality with the following:

```bash
enroot import docker://alpine:latest
```

<!-- Links -->

[enroot]: https://github.com/NVIDIA/enroot
[pyxis]: https://github.com/NVIDIA/pyxis
[spank]: https://slurm.schedmd.com/spank.html
