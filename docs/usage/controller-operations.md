# Controller Operations

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Controller Operations](#controller-operations)
  - [Table of Contents](#table-of-contents)
  - [Persistence](#persistence)
  - [High Availability](#high-availability)

<!-- mdformat-toc end -->

## Persistence

Once the Controller CR is initially installed, the `persistence` configuration
cannot be changed.

## High Availability

The Slurm Operator has support for [Slurm High Availability (HA)][slurm-ha]
configuration.

Slurm Controller High Availability (HA) can be enabled on the Controller CR by
setting `ha.enabled=true` and `persistence.existingClaim` to a PVC with
ReadWriteMany (RWX) access mode.

<!-- Links -->

[slurm-ha]: https://slurm.schedmd.com/quickstart_admin.html#HA
