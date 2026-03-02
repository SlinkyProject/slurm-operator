# NodeSet Controller

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [NodeSet Controller](#nodeset-controller)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Design](#design)
    - [Sequence Diagram](#sequence-diagram)
  - [Drain and Cordon](#drain-and-cordon)
    - [Kubernetes to Slurm (K8s → Slurm)](#kubernetes-to-slurm-k8s--slurm)
    - [Slurm to Kubernetes (Slurm → K8s)](#slurm-to-kubernetes-slurm--k8s)
    - [Loop Prevention](#loop-prevention)
  - [Well-Known Annotations](#well-known-annotations)
    - [Pod Annotations](#pod-annotations)
    - [Node Annotations](#node-annotations)

<!-- mdformat-toc end -->

## Overview

The nodeset controller is responsible for managing and reconciling the NodeSet
CRD, which represents a set of homogeneous Slurm Nodes.

## Design

This controller is responsible for managing and reconciling the NodeSet CRD. In
addition to the regular responsibility of managing resources in Kubernetes via
the Kubernetes API, this controller should take into consideration the state of
Slurm to make certain reconciliation decisions.

### Sequence Diagram

```mermaid
sequenceDiagram
    autonumber

    actor User as User
    participant KAPI as Kubernetes API
    participant NS as NodeSet Controller
    box Operator Internals
        participant SCM as Slurm Client Map
        participant SEC as Slurm Event Channel
    end %% Operator Internals
    participant SC as Slurm Client
    participant SAPI as Slurm REST API

    loop Watch Slurm Nodes
        SC->>+SAPI: Get Slurm Nodes
        SAPI-->>-SC: Return Slurm Nodes
        SC->>SEC: Add Event for Cache Delta
    end %% loop Watch Slurm Nodes

    note over KAPI: Handle CR Update
    SEC-->>NS: Watch Event Channel
    User->>KAPI: Update NodeSet CR
    KAPI-->>NS: Watch NodeSet CRD
    opt Scale-out Replicas
        NS->>KAPI: Create Pods
    end %% Scale-out Replicas
    opt Scale-in Replicas
        SCM-->>NS: Lookup Slurm Client
        NS->>+SC: Drain Slurm Node
        SC->>+SAPI: Drain Slurm Node
        SAPI-->>-SC: Return Drain Slurm Node Status
        SC-->>-NS: Drain Slurm Node
        alt Slurm Node is Drained
            NS->>KAPI: Delete Pod
        else
            NS->>NS: Check Again Later
        end %% alt Slurm Node is Drained
    end %% opt Scale-in Replicas
```

## Drain and Cordon

The NodeSet controller synchronizes drain state **bidirectionally** between
Kubernetes and Slurm. Draining a node on either side is reflected on the other.

### Kubernetes to Slurm (K8s → Slurm)

When a Kubernetes node is cordoned (`kubectl cordon <node>`), the NodeSet
controller:

1. Sets the `nodeset.slinky.slurm.net/pod-cordon: "true"` annotation on each
   NodeSet pod running on that node.
2. Drains the corresponding Slurm node via the Slurm REST API, prefixing the
   reason with `slurm-operator:`.

When the Kubernetes node is uncordoned, the pod annotation is removed and the
Slurm node is undrained.

The same flow applies when the `pod-cordon` annotation is set directly on a
NodeSet pod (e.g. for targeted draining of a single Slurm node).

A custom drain reason can be provided by setting the
`nodeset.slinky.slurm.net/node-cordon-reason` annotation on the Kubernetes
node before cordoning it.

### Slurm to Kubernetes (Slurm → K8s)

When a Slurm node is drained externally (e.g. via `scontrol update
node=<name> state=drain reason="maintenance"`), the NodeSet controller detects
this during its periodic reconciliation and:

1. Sets `nodeset.slinky.slurm.net/pod-cordon: "true"` on the corresponding
   pod.
2. Sets `nodeset.slinky.slurm.net/pod-cordon-source: "slurm"` to record that
   the cordon originated from the Slurm side.
3. Sets `nodeset.slinky.slurm.net/pod-cordon-reason` to the Slurm node's
   drain reason, making it visible from `kubectl describe pod`.

When the Slurm node is undrained externally, the controller removes all three
annotations from the pod.

The operator distinguishes external drains from its own by checking the Slurm
node reason prefix (`slurm-operator:`). Drains that do not carry this prefix
are treated as externally initiated.

### Loop Prevention

A bidirectional sync must avoid infinite loops where each side re-triggers the
other. The operator prevents this using the `pod-cordon-source` annotation:

- When the controller sets `pod-cordon` because of an external Slurm drain, it
  also sets `pod-cordon-source: "slurm"`.
- On subsequent reconciles, if the pod is cordoned with source `"slurm"`, the
  controller does **not** issue a drain command to Slurm (the node is already
  drained).
- If the Slurm node is later undrained, the controller detects the state change
  and removes the pod annotations.
- Cordons that originate from the Kubernetes side (node cordon or direct
  annotation) do **not** set a source, so they continue to trigger the normal
  K8s → Slurm drain flow.

## Well-Known Annotations

### Pod Annotations

| Annotation | Value | Description |
|---|---|---|
| `nodeset.slinky.slurm.net/pod-cordon` | `"true"` | Marks the pod for Slurm node drain. When set, the corresponding Slurm node is drained (or already drained, in the Slurm → K8s direction). |
| `nodeset.slinky.slurm.net/pod-cordon-source` | `"slurm"` | Present only when the cordon was initiated from the Slurm side (external drain). Prevents the operator from re-draining the Slurm node. |
| `nodeset.slinky.slurm.net/pod-cordon-reason` | string | The Slurm node drain reason, stored when the cordon originates from an external Slurm drain. |
| `nodeset.slinky.slurm.net/pod-deletion-cost` | integer | Influences pod deletion order during scale-in. Lower cost pods are deleted first. |
| `nodeset.slinky.slurm.net/pod-deadline` | RFC 3339 timestamp | Indicates when the Slurm node's running workload is expected to complete. Earlier deadlines are preferred for deletion. |

### Node Annotations

| Annotation | Value | Description |
|---|---|---|
| `nodeset.slinky.slurm.net/node-cordon-reason` | string | When set on a Kubernetes node before cordoning, overrides the default Slurm drain reason used by the operator. |
| `topology.slinky.slurm.net/line` | string | The Slurm dynamic topology line (e.g. `"topo-switch:s2,topo-block:b2"`). See [Topology](../usage/topology.md). |
