# Topology

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Topology](#topology)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Kubernetes](#kubernetes)
  - [Slurm](#slurm)
  - [Example](#example)

<!-- mdformat-toc end -->

## Overview

The operator can propagate topology from Kubernetes node to Slurm nodes (those
running as NodeSet pods). When a NodeSet pod is running on a Kubernetes node,
its annotations are used by the operator to update the registered Slurm node's
topology. A topology file is required for dynamic topology to work.

If there is a misconfiguration of `topology.yaml` or the Kubernetes node
annotation, an error will be reported in the operator logs.

## Kubernetes

Each Kubernetes node should be annotated with `topology.slinky.slurm.net/line`.
its value is transparently used by the operator to update Slurm node topology
information, only if running as a NodeSet pod.

For example, the following Kubernetes Node snippet has the Slinky topology
annotation applied.

```yaml
apiVersion: v1
kind: Node
metadata:
  annotations:
    topology.slinky.slurm.net/line: topo-switch:s0,topo-block:b0
  name: node0
```

## Slurm

Slurm supports [topology.yaml], a YAML based configuration file capable of
expressing one or more topology configurations in the same Slurm cluster.

Please review the [Slurm topology guide][topology-guide].

## Example

For example, your Slurm cluster has the following `topology.yaml`.

```yaml
---
- topology: topo-switch
  cluster_default: true
  tree:
    switches:
      - switch: sw_root
        children: s[1-2]
      - switch: s1
        nodes: node[1-2]
      - switch: s2
        nodes: node[3-4]
- topology: topo-block
  cluster_default: false
  block:
    block_sizes:
      - 2
      - 4
    blocks:
      - block: b1
        nodes: node[1-2]
      - block: b2
        nodes: node[3-4]
- topology: topo-flat
  cluster_default: false
  flat: true
```

And your Kubernetes nodes were annotated as such to match the `topology.yaml`.

```yaml
---
apiVersion: v1
kind: Node
metadata:
  annotations:
    topology.slinky.slurm.net/line: topo-switch:s1,topo-block:b1
  name: node1
---
apiVersion: v1
kind: Node
metadata:
  annotations:
    topology.slinky.slurm.net/line: topo-switch:s1,topo-block:b1
  name: node2
---
apiVersion: v1
kind: Node
metadata:
  annotations:
    topology.slinky.slurm.net/line: topo-switch:s2,topo-block:b2
  name: node3
---
apiVersion: v1
kind: Node
metadata:
  annotations:
    topology.slinky.slurm.net/line: topo-switch:s2,topo-block:b2
  name: node4
```

Then when the `slinky-0` NodeSet pod is scheduled onto Kubernetes node `node3`,
the operator will update the Slurm node's topology to match that of
`topology.slinky.slurm.net/line`. Hence Slurm will report the following after
the Slurm node's topology was updated.

```sh
$ scontrol show nodes slinky-0 | grep -Eo "NodeName=[^ ]+|[ ]*Comment=[^ ]+|[ ]*Topology=[^ ]+"
NodeName=slinky-0
   Comment={"namespace":"slurm","podName":"slurm-worker-slinky-0","node":"node3"}
   Topology=topo-switch:s2,topo-block:b2
```

<!-- Links -->

[topology-guide]: https://slurm.schedmd.com/topology.html
[topology.yaml]: https://slurm.schedmd.com/topology.yaml.html
