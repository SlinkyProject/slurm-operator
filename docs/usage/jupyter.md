# JupyterLab Deployment Guide

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [JupyterLab Deployment Guide](#jupyterlab-deployment-guide)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Installing JupyterLab within the `slurmd` Images](#installing-jupyterlab-within-the-slurmd-images)
  - [Modifying the Slurm Helm chart's values for JupyterLab](#modifying-the-slurm-helm-charts-values-for-jupyterlab)
  - [Launching JupyterLab Using `sbatch`](#launching-jupyterlab-using-sbatch)
  - [Accessing a JupyterLab Instance Running in a Slinky Cluster](#accessing-a-jupyterlab-instance-running-in-a-slinky-cluster)

<!-- mdformat-toc end -->

## Overview

[JupyterLab] is a highly extensible, feature-rich notebook authoring application
and editing environment. JupyterLab has proven to be a reliable and
easy-to-configure solution for those wishing to deploy an interactive computing
environment with Slurm-operator.

This documentation provides guidance on the installation and deployment of
JupyterLab on a Slurm cluster running within Kubernetes using Slurm-operator.

## Installing JupyterLab within the `slurmd` Images

The Slurm Helm Chart relies upon a number of [container images] published by
SchedMD. These images can be modified as needed by sites running Slurm-operator
in order to provide custom programming environments for their users. More
information on those container images can be found in the
[containers repo][container images].

To run JupyterLab on a Slinky cluster, modify the image used by a worker
nodeset's slurmd pods to install Python and the JupyterLab software. Per the
[image build documentation] for the Slinky containers, these modifications
should be made in the `base-extra` container layer.

1. Clone the [Slinky Container Repository]:

```bash
git clone git@github.com:SlinkyProject/containers.git
cd containers
```

2. Edit the `base-extra` layer within the Dockerfile of your preferred version
   and release to install the JupyterLab pip package. Directories for each
   version and flavor can be found within the `schedmd/slurm/` directory of the
   containers repository.

To install JupyterLab within the `rockylinux9` images, the `base-extra` section
can be modified as follows:

```dockerfile
################################################################################
FROM base AS base-extra

SHELL ["bash", "-c"]

RUN --mount=type=cache,target=/var/cache/dnf,sharing=locked <<EOR
# Install Extra Packages
set -xeuo pipefail
dnf -q -y install \
  python3
dnf -q -y install \
  python3-pip
pip install jupyterlab
EOR

################################################################################
```

3. Once modifications have been made to the `base-extra` container layer, the
   `slurmd` image can be built with these changes included:

```bash
export VERSION=25.11
export FLAVOR="rockylinux9"
export BAKE_IMPORTS="--file ./docker-bake.hcl --file ./$VERSION/$FLAVOR/slurm.hcl"
docker bake $BAKE_IMPORTS slurmd
```

After the image build has completed, a tag should be applied to distinguish the
image from the images published by SchedMD:

```console
$ docker image ls | head -n 2
REPOSITORY                                                                       TAG                                                                IMAGE ID       CREATED          SIZE
ghcr.io/slinkyproject/slurmd                                                     25.11-rockylinux9                                                  9434c2d4fae4   43 seconds ago   712MB

docker tag 9434c2d4fae4 ghcr.io/slinkyproject/slurmd:jupyterlab
```

4. Next, the modified image will need to be uploaded to a container registry
   within your environment. The method varies based on cloud provider and
   software stack.

## Modifying the Slurm Helm chart's values for JupyterLab

The Slurm Helm chart's `values.yaml` file must be modified for deploying
JupyterHub. Specifically, the values of
`nodesets.slinky.slurmd.image.repository`, `nodesets.slinky.slurmd.image.tag`
must be changed to reference the registry by which your images are provided and
`nodesets.slinky.slurmd.ports` must be modified to expose the port on which
JupyterLab will be served.

A minimal `values.yaml` for running JupyterLab within a Slurm-operator
environment:

```yaml
nodesets:
  slinky:
    enabled: true
    replicas: 1
    slurmd:
      image:
        repository: ghcr.io/slinkyproject/slurmd
        tag: jupyterlab
      ports:
        - containerPort: 9999
```

## Launching JupyterLab Using `sbatch`

Within Slurm, the [`sbatch`] command is used to submit batch jobs using a
script. Below is a basic [`sbatch`] script that can be used to launch JupyterLab
on a Slinky cluster:

```bash
#!/bin/bash
#SBATCH --job-name=jupyterlab-singleuser
#SBATCH --account=vivian
#SBATCH --nodes=1
#SBATCH --time=01:00:00
#
## Launch the JupyterLab server:
jupyter lab --port=9999 --no-browser
```

## Accessing a JupyterLab Instance Running in a Slinky Cluster

After submitting the above sbatch script, after the resultant job has been
allocated resources, an instance of JupyterHub will be served on port 9999 of
the Slurm worker pod on which it was scheduled.

[`kubectl port-forward`] provides a means by which local ports can be forwarded
to ports on a pod. To access a JupyterLab instance running on port 9999 of the
`slurm-worker-slinky-0` pod, the following command can be used:

```bash
kubectl port-forward -n slurm slurm-worker-slinky-0 8081:9999
```

The JupyterLab instance running within that pod will now be accessible at
http://localhost:8081 in your browser.

<!-- Links -->

[container images]: https://github.com/orgs/SlinkyProject/packages
[image build documentation]: https://slinky.schedmd.com/projects/containers/en/latest/build.html#extending-the-images-with-additional-software
[jupyterlab]: https://jupyterlab.readthedocs.io/en/stable/
[slinky container repository]: https://github.com/SlinkyProject/containers
[`kubectl port-forward`]: https://kubernetes.io/docs/reference/kubectl/generated/kubectl_port-forward/
[`sbatch`]: https://slurm.schedmd.com/sbatch.html
