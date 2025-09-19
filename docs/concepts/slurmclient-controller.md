# Slurm Client Controller

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Slurm Client Controller](#slurm-client-controller)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Sequence Diagram](#sequence-diagram)

<!-- mdformat-toc end -->

## Overview

This controller is responsible for managing Slurm Clients used by other internal
controllers.

This controller uses the [Slurm client] library.

## Sequence Diagram

```mermaid
sequenceDiagram
    autonumber

    actor User as User
    participant KAPI as Kubernetes API
    participant SCC as SlurmClient Controller
    box Operator Internals
        participant SCM as Slurm Client Map
        participant SEC as Slurm Event Channel
    end %% Operator Internals

    note over KAPI: Handle CR Creation
    User->>KAPI: Create Controller CR
    KAPI-->>SCC: Watch Controller CRD
    create participant SC as Slurm Client
    SCC->>+SC: Create Slurm Client
    SC-->>-SCC: Return Slurm Client Status
    loop Watch Slurm Nodes
        SC->>+SAPI: Get Slurm Nodes
        SAPI-->>-SC: Return Slurm Nodes
        SC->>SEC: Add Event for Cache Delta
    end %% loop Watch Slurm Nodes
    SCC->>SCM: Add Slurm Client to Map
    SCC->>+SC: Ping Slurm Control Plane
    SC->>+SAPI: Ping Slurm Control Plane
    SAPI-->>-SC: Return Ping
    SC-->>-SCC: Return Ping
    SCC->>KAPI: Update Controller CR Status

    note over KAPI: Handle CR Deletion
    User->>KAPI: Delete Controller CR
    KAPI-->>SCC: Watch Controller CRD
    SCM-->>SCC: Lookup Slurm Client
    SCC->>SCM: Remove Slurm Client from Map
    destroy SC
    SCC-)SC: GC Slurm Client

    participant SAPI as Slurm REST API
```

<!-- Links -->

[slurm client]: https://github.com/SlinkyProject/slurm-client
