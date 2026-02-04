# Usage Tutorial

## Overview

This guide provides an example workflow for a simple use of Pytorch running on
Slurm using Slurm Operator. By the end of this tutorial, the
[Graph Convolutional Network] Pytorch example should be running on a Slurm
cluster within Kubernetes.

> [!WARNING]
> These instructions are not intended for use in a production environment.

## Prerequisites

- A deployment of `slurm-operator` with a LoginSet enabled.
- The latest official release of the following repositories cloned locally:
  - Slurm Operator: [slurm-operator]
  - Slinky Containers: [containers]

## Building Images

In-depth instructions can be found at [Building Slinky Containers].

The Slinky project uses Docker Buildx Bake to build modular images for each
Slurm component. In order to extend or change functionality, the Dockerfile
containing the images' build configuration must be modified. For example, the
configuration for images built with Slurm 25.11 and Rocky Linux 9 can be found
within the [containers] repo at:
`containers/schedmd/slurm/25.11/rockylinux9/Dockerfile`.

To install Pytorch for this example, change the packages that are installed by
`dnf` in the `base-extra` layer. This can be done by appending the required
Python and Pytorch packages to the list, cloning the Pytorch repository, and
installing the GCN example's dependencies from within its directory via pip. An
example of how this is done is the following:

```dockerfile
################################################################################
FROM base AS base-extra

SHELL ["bash", "-c"]

RUN --mount=type=cache,target=/var/cache/dnf,sharing=locked <<EOR
# Install Extra Packages
set -xeuo pipefail
dnf -q -y install --setopt='install_weak_deps=False' \
  openmpi python3-pip python-setuptools python3-scipy
EOR

# pytorch
RUN git clone --depth=2 https://github.com/pytorch/examples.git
WORKDIR /tmp/examples/gcn
RUN pip install -r requirements.txt

################################################################################
```

After making modifications, images can be tagged by setting the `SUFFIX`
environment variable, which appends a `-<SUFFIX>` to the tag of the version you
choose to build. Then, from the container repo's root directory, the images can
be built by running the following command, where `<version>` must be replaced
with the Slurm version, and `<distribution>` with the distribution desired:

```bash
docker bake --file containers/schedmd/slurm/docker-bake.hcl --file containers/schedmd/slurm/<version>/<distribution>/slurm.hcl
```

After building the images, they must be made available on your cluster via a
container registry.

## Loading

The Slurm Helm chart's [values file] can be downloaded [here][slurm-values], or
can be found in the [slurm-operator] repo at `helm/slurm/values.yaml`. The
values at `nodesets.slinky.slurmd.image`, where `slinky` is replaced by the name
of your NodeSet, can be used to specify the repository and tag of the image used
by Slurm Operator to deploy that NodeSet's slurmd pods. These values must be
modified to reference the container registry within which the images built in
the previous step are hosted and the tag that was applied to them.

> [!NOTE]
> Take note of the `ImagePullPolicy` that is [set by default by Kubernetes].

## Running

The functionality can be tested by running Pytorch within Slurm. Create the
following sbatch script as the file `pytorch-sbatsh.sh`:

```bash
#!/bin/bash
# Slurm Parameters
#SBATCH -n 3                      # Run n tasks
#SBATCH -N 3                      # Run across N nodes
#SBATCH -t 0-00:10                # Time limit in D-HH:MM
#SBATCH -p slinky                 # Partition to submit to
#SBATCH --mem=100                 # Memory pool for all cores
#SBATCH -o pytorch-output_%j.out  # File to which STDOUT will be written, %j inserts jobid
#SBATCH -e pytorch-errors_%j.err  # File to which STDERR will be written, %j inserts jobid

echo "This was run on $SLURM_JOB_NODELIST."
srun -D /tmp/examples/gcn python3 main.py --epochs 200 --lr 0.01 --l2 5e-4 --dropout-p 0.5 --hidden-dim 16 --val-every 20 --include-bias
```

This script runs the GCN Pytorch example on three nodes, modify it or scale
accordingly. It must be copied to your `slurm-login-slinky` pod, this can be
done with the following command, where `<hash>` is replaced with the hash of the
current slurm-login-slinky pod:

```bash
kubectl -n slurm cp ~/location/of/script/pytorch-sbatch.sh slurm-login-slinky-<hash>:/root/
```

Then the script can be run from the slurm-login-slinky node with:

```bash
sbatch pytorch-sbatch.sh
```

The output should be available in a file titled `pytorch-output_<JOB>.out`,
created on the worker node on which it ran. Any errors would be in
`pytorch-errors_<JOB>.err`. To retrieve these files, they can be copied from the
`slurm-worker` node with the following command, where `<N>` is replaced by the
ordinal of the pod on which the job ran, and `<JOB>` is replaced by the jobid:

```sh
kubectl -n slurm cp slurm-worker-slinky-<N>:/root/pytorch-output_<JOB>.out .
```

For further reading, see the [Slinky] and [Slurm] documentation.

<!-- Links -->

[building slinky containers]: https://slinky.schedmd.com/projects/containers/en/latest/build.html
[containers]: https://github.com/SlinkyProject/containers
[graph convolutional network]: https://github.com/pytorch/examples/tree/main/gcn
[set by default by kubernetes]: https://kubernetes.io/docs/concepts/containers/images/#updating-images
[slinky]: https://slinky.schedmd.com/en/latest/
[slurm]: https://slurm.schedmd.com/documentation.html
[slurm-operator]: https://github.com/SlinkyProject/slurm-operator
[slurm-values]: https://raw.githubusercontent.com/SlinkyProject/slurm-operator/refs/tags/v1.0.0/helm/slurm/values.yaml
[values file]: https://helm.sh/docs/chart_template_guide/values_files/
