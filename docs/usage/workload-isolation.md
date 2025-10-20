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

By default, Slurm-operator does not taint nodes on which NodeSet pods are
running. This can be configured by setting `taintKubeNodes` to `true` for
specific NodeSets in the Slurm Helm chart, or by deploying a NodeSet CR with
`TaintKubeNodes: true`. This option causes
`nodeset.slinky.slurm.net/worker=:NoExecute` taints to be applied to any node on
which a NodeSet slurmd replica is scheduled. Any pods that do not have a
toleration matching this taint will be evicted by the Kubernetes controllers.

This document provides an example of how this can be done manually using
[taints and tolerations], to demonstrate what Slurm-operator does for NodeSet
pods automatically when `taintKubeNodes` is set.

## Pre-requisites

This guide assumes that the user has access to a functional Kubernetes cluster
running `slurm-operator`. See the [quickstart guide] for details on setting up
`slurm-operator` on a Kubernetes cluster.

## Taints and Tolerations

Taints are a mechanism that Kubernetes provides that allows a node to repel a
set of pods that lack a matching toleration. Tolerations are the mechanism that
Kubernetes provides that allow the scheduler to schedule pods on nodes with
matching taints.

Apply a taint to the nodes that will only run Slurm pods:

```bash
kubectl taint nodes kind-worker2 slinky.slurm.net/slurm:NoExecute
kubectl taint nodes kind-worker3 slinky.slurm.net/slurm:NoExecute
kubectl taint nodes kind-worker4 slinky.slurm.net/slurm:NoExecute
kubectl taint nodes kind-worker5 slinky.slurm.net/slurm:NoExecute
```

Confirm that the taint was applied:

```bash
kubectl get nodes -o jsonpath="{range .items[*]}{.metadata.name}:{' '}{range .spec.taints[*]}{.key}={.value}:{.effect},{' '}{end}{'\n'}{end}"

kind-control-plane: node-role.kubernetes.io/control-plane=:NoSchedule,
kind-worker:
kind-worker2: slinky.slurm.net/slurm=:NoExecute
kind-worker3: slinky.slurm.net/slurm=:NoExecute
kind-worker4: slinky.slurm.net/slurm=:NoExecute
kind-worker5: slinky.slurm.net/slurm=:NoExecute
```

Next, configure the tolerations on the `slurm-operator` components. Each of the
components of `slurm-operator` can have their `tolerations` set from within
`values.yaml`. Update the `tolerations` section of all components to match the
`taint` that you applied in step 1. This will need to be done for all components
in both the `slurm` and `slurm-operator` Helm charts.

```yaml
  # -- Tolerations for pod assignment.
  # Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
  tolerations:
    - key: slinky.slurm.net/slurm
      operator: Exists
      effect: NoSchedule
```

## Pod Anti-Affinity

In some cases [anti-affinity] must be configured in order to prevent multiple
NodeSet pods (slurmd) from being scheduled on the same node. Pod [anti-affinity]
can be configured under the `affinity` section of a NodeSet. To ensure that
multiple NodeSet pods cannot be scheduled on the same node, add the following to
the `affinity` section:

```yaml
nodesets:
  slinky:
    ...
    # -- Affinity for pod assignment.
    # Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity
    affinity:
      podAntiAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
        - topologyKey: kubernetes.io/hostname
          labelSelector:
            matchExpressions:
            - key: app.kubernetes.io/name
              operator: In
              values:
              - slurmctld
              - slurmdbd
              - slurmrestd
              - mariadb
              - slurmd
```

After applying the Helm chart with `affinity` set in `values.yaml`, the
`affinity` section can be observed in the `NodeSet` by running:

```bash
kubectl describe NodeSet --namespace slurm
```

<!-- links -->

[anti-affinity]: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
[quickstart guide]: ../installation.md
[taints and tolerations]: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
