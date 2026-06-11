# NodeSet Controller

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [NodeSet Controller](#nodeset-controller)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Design](#design)
    - [Sequence Diagram](#sequence-diagram)
    - [Slurm Node State Visibility](#slurm-node-state-visibility)
    - [Drain and Cordon Design](#drain-and-cordon-design)
    - [Scale-in Lifecycle](#scale-in-lifecycle)
  - [Config-Change Rollouts](#config-change-rollouts)

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
    end %% Operator Internals
    participant SC as Slurm Client
    participant SAPI as Slurm REST API

    note over KAPI: Handle CR Update
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

### Slurm Node State Visibility

The operator projects Slurm node states onto Kubernetes pod conditions so that
external tools can observe Slurm state without querying the Slurm REST API
directly. Every condition type uses the prefix `SlurmNodeState`.

**Base states** — exactly one is active at a time:

| Condition Type            | Slurm State |
| ------------------------- | ----------- |
| `SlurmNodeStateAllocated` | Allocated   |
| `SlurmNodeStateDown`      | Down        |
| `SlurmNodeStateError`     | Error       |
| `SlurmNodeStateFuture`    | Future      |
| `SlurmNodeStateIdle`      | Idle        |
| `SlurmNodeStateMixed`     | Mixed       |
| `SlurmNodeStateUnknown`   | Unknown     |

**Flag states** — combinable with base states:

| Condition Type                | Slurm Flag    |
| ----------------------------- | ------------- |
| `SlurmNodeStateCompleting`    | Completing    |
| `SlurmNodeStateDrain`         | Drain         |
| `SlurmNodeStateFail`          | Fail          |
| `SlurmNodeStateInvalid`       | Invalid       |
| `SlurmNodeStateInvalidReg`    | InvalidReg    |
| `SlurmNodeStateMaintenance`   | Maintenance   |
| `SlurmNodeStateNotResponding` | NotResponding |
| `SlurmNodeStateUndrain`       | Undrain       |

The `.Message` field on the `SlurmNodeStateDrain` condition carries the Slurm
drain reason string.

**Conceptual states** derived from condition combinations:

- **Busy** — the node is running work: `Allocated`, `Mixed`, or `Completing` is
  `True`.
- **Drain** — the node has the drain flag set: `Drain` is `True` AND `Undrain`
  is not `True`.
- **Drained** — drain is complete: `Drain` AND NOT `Busy`.
- **Draining** — drain is in progress: `Drain` AND `Busy`.

### Drain and Cordon Design

The operator implements a unidirectional Kubernetes-to-Slurm cordon model.
Cordoning a Kubernetes node or annotating a pod triggers a Slurm drain;
uncordoning reverses it. The operator never initiates a cordon on its own — it
only reacts to external signals.

```mermaid
flowchart TD
    Start["For each NodeSet pod"] --> CheckExternal{"External reason OR unresponsive?"}
    CheckExternal -->|Yes| Skip["Skip — preserve external state"]
    CheckExternal -->|No| CheckNodeCordon{"K8s node cordoned?"}
    CheckNodeCordon -->|Yes| PropagateCordon["Cordon pod + drain Slurm node"]
    CheckNodeCordon -->|No| CheckPodCordon{"pod-cordon=true?"}
    CheckPodCordon -->|Yes| DrainSlurm["Drain Slurm node"]
    CheckPodCordon -->|No| UndrainSlurm["Undrain Slurm node"]
```

The operator prefixes all drain reasons it sets with `slurm-operator:`. Drain
reasons without this prefix are treated as externally owned and are never
modified or cleared. This ensures that drains set by administrators or external
tools via `scontrol` are preserved across reconciliation cycles.

### Scale-in Lifecycle

During scale-in, the controller selects pods for deletion using a multi-criteria
sort order (see
[Influencing Scale-in Order](../usage/nodeset-operations.md#influencing-scale-in-order))
and then drains each selected pod's Slurm node before deleting it. A pod is
never deleted until its Slurm node is fully drained, ensuring running jobs are
not interrupted.

For practical usage of annotations, labels, and integration patterns, see
[NodeSet Operations](../usage/nodeset-operations.md).

## Config-Change Rollouts

A NodeSet can automatically roll its worker pods when a mounted Secret or
ConfigMap changes. Enable it by setting this annotation on the NodeSet:

```yaml
metadata:
  annotations:
    slinky.slurm.net/reload-on-change: "true"
```

When enabled, the controller tracks every Secret and ConfigMap referenced by the
pod template through volumes (including projected sources), `envFrom`, and
`env[].valueFrom`. It computes a checksum for each and stamps it as a
per-resource annotation (`slinky.slurm.net/secret-<name>-hash`,
`slinky.slurm.net/configmap-<name>-hash`) on the pod template. A content change
updates the checksum, which produces a new revision and triggers a rolling
update following the NodeSet's `updateStrategy` (including Slurm node draining
and `maxUnavailable`).

> [!NOTE]
> For very long resource names, the per-resource annotation key is truncated
> with a short disambiguator to stay within Kubernetes annotation key limits.

The most recently observed checksums are recorded in the NodeSet status under
`status.configHashes`, keyed by `<kind>/<name>` (for example,
`configmap/my-config`). On each reconcile the controller compares the freshly
computed checksums against this map; when a tracked resource's content has
changed it emits a `ConfigHashChanged` event against the NodeSet before rolling
the pods. No event is emitted the first time a resource is observed (such as on
creation or when the feature is first enabled), only on subsequent changes.

Operator-injected resources (the Slurm auth key, SSH config) are not covered by
this annotation; they are handled separately.
