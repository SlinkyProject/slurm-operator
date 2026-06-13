# Node Features

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Node Features](#node-features)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Kubernetes](#kubernetes)
  - [Slurm](#slurm)
  - [Example](#example)

<!-- mdformat-toc end -->

## Overview

The operator can propagate node features from Kubernetes nodes to Slurm nodes
(those running as NodeSet pods). When a NodeSet pod runs on a Kubernetes node, each
value in the node's `features.slinky.slurm.net/spec` annotation is applied to the
registered Slurm node's available and active features under a reserved prefix,
`k8s/` (for example the annotation `nn-75bfcf47ca3e4f7dc` becomes the Slurm feature
`k8s/nn-75bfcf47ca3e4f7dc`).

Use the annotation for per-node attributes discovered at runtime, such as a network
switch ID. NodeSet-wide features (the NodeSet name and any `Feature=`/`Features=`
entries in `extraConf`) are seeded directly by slurmd at registration via `--conf`
and are not prefixed.

The operator owns only the `k8s/` namespace. On each reconcile it replaces the
prefixed features with the current annotation values and preserves every other
feature, including the NodeSet baseline, `extraConf` features, and features managed
outside the operator such as those from a Slurm `NodeFeaturesPlugins` plugin or a
manual `scontrol update`. Removing the annotation clears the node's `k8s/` features
while leaving the rest intact.

## Kubernetes

Annotate each Kubernetes node with `features.slinky.slurm.net/spec` and a
comma-separated list of feature names. Each token is used verbatim and applied to
the Slurm node under the `k8s/` prefix. Only Slurm nodes backed by a NodeSet pod
scheduled on the node are updated.

For example, the following Kubernetes Node snippet has the Slinky node features
annotation applied.

```yaml
apiVersion: v1
kind: Node
metadata:
  name: node0
  annotations:
    features.slinky.slurm.net/spec: nn-75bfcf47ca3e4f7dc
```

The annotation is normally written by an external component that discovers each
node's features (for example, a daemon querying a cloud provider's network
topology API). The operator only propagates the annotation it finds.

## Slurm

Slurm node features can be requested by a job with `--constraint`. See the Slurm
[Features][features-conf] documentation. Because operator-managed features carry
the `k8s/` prefix, jobs target them as `--constraint=k8s/<feature>`. The operator
applies each prefixed feature to both available and active features, and because it
only touches the `k8s/` namespace it does not interfere with features managed by a
`NodeFeaturesPlugins` plugin.

## Example

Suppose the NodeSet is named `slinky` and sets `extraConf: Features=GPU`. slurmd
registers the node with the baseline features `slinky` and `GPU` (unprefixed). The
Kubernetes node where the `slinky-0` NodeSet pod runs is annotated with its
leaf-switch ID.

```yaml
---
apiVersion: v1
kind: Node
metadata:
  name: node0
  annotations:
    features.slinky.slurm.net/spec: nn-75bfcf47ca3e4f7dc
```

When the `slinky-0` NodeSet pod is scheduled onto Kubernetes node `node0`, the
operator applies the annotation under the `k8s/` prefix, preserving the baseline.
Slurm then reports both the baseline and the prefixed feature.

```console
$ scontrol show node slinky-0 | grep -Eo "NodeName=[^ ]+|[ ]*AvailableFeatures=[^ ]+|[ ]*ActiveFeatures=[^ ]+"
NodeName=slinky-0
   AvailableFeatures=GPU,k8s/nn-75bfcf47ca3e4f7dc,slinky
   ActiveFeatures=GPU,k8s/nn-75bfcf47ca3e4f7dc,slinky
```

A job can then be constrained to that switch.

```sh
sbatch --constraint=k8s/nn-75bfcf47ca3e4f7dc ...
```

<!-- Links -->

[features-conf]: https://slurm.schedmd.com/slurm.conf.html#OPT_Features
