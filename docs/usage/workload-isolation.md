# Workload Isolation

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Workload Isolation](#workload-isolation)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Pre-requisites](#pre-requisites)
  - [Taints and Tolerations](#taints-and-tolerations)
  - [Pod Anti-Affinity](#pod-anti-affinity)

<!-- mdformat-toc end -->

## Overview

When running Slinky in certain environments, it may be necessary to isolate the
nodes running Slurm NodeSets from other Kubernetes workloads.

This document provides an example of how this can be done manually using
[taints and tolerations].

## Pre-requisites

This guide assumes that the user has access to a functional Kubernetes cluster
running `slurm-operator`. See the [quickstart guide] for details on setting up
`slurm-operator` on a Kubernetes cluster.

## Taints and Tolerations

Taints are a mechanism that Kubernetes provides that allows a node to repel a
set of pods that lack a matching toleration. Tolerations are the mechanism that
Kubernetes provides that allow the scheduler to schedule pods on nodes with
matching taints.

Label and taint the nodes dedicated to Slurm workers:

```bash
kubectl label nodes kind-worker2 kind-worker3 kind-worker4 kind-worker5 \
  slinky.slurm.net/role=worker
kubectl taint nodes kind-worker2 kind-worker3 kind-worker4 kind-worker5 \
  slinky.slurm.net/slurm=worker:NoSchedule
```

Confirm that the taint was applied:

```console
$ kubectl get nodes -o jsonpath="{range .items[*]}{.metadata.name}:{' '}{range .spec.taints[*]}{.key}={.value}:{.effect},{' '}{end}{'\n'}{end}"
kind-control-plane: node-role.kubernetes.io/control-plane=:NoSchedule,
kind-worker:
kind-worker2: slinky.slurm.net/slurm=worker:NoSchedule
kind-worker3: slinky.slurm.net/slurm=worker:NoSchedule
kind-worker4: slinky.slurm.net/slurm=worker:NoSchedule
kind-worker5: slinky.slurm.net/slurm=worker:NoSchedule
```

Next, configure the NodeSet's pod template to select and tolerate the dedicated
nodes. The toleration permits scheduling onto the tainted nodes; the node
selector prevents these worker pods from also scheduling onto general-purpose
nodes.

```yaml
nodesets:
  slinky:
    podSpec:
      nodeSelector:
        slinky.slurm.net/role: worker
      tolerations:
        - key: slinky.slurm.net/slurm
          operator: Equal
          value: worker
          effect: NoSchedule
```

## Pod Anti-Affinity

By default, the NodeSet controller adds required pod anti-affinity on
`kubernetes.io/hostname`, preventing more than one slurmd pod from being
scheduled on a Kubernetes node. This is controlled by `oversubscribeNode`, whose
default is `false`; no additional affinity configuration is needed.

To deliberately permit multiple NodeSet pods on the same Kubernetes node, set:

```yaml
nodesets:
  slinky:
    oversubscribeNode: true
```

After applying the Helm chart, the controller-generated pod anti-affinity can be
observed on a worker pod by running:

```bash
kubectl get pod --namespace slurm <worker-pod> -o yaml
```

<!-- links -->

[quickstart guide]: ../installation.md
[taints and tolerations]: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
