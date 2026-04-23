# Upgrade

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Upgrade](#upgrade)
  - [Table of Contents](#table-of-contents)
  - [From 1.0.x to 1.1.x](#from-10x-to-11x)
  - [From 1.1.x to 1.2.x](#from-11x-to-12x)
    - [Shared defaults for loginsets and nodesets](#shared-defaults-for-loginsets-and-nodesets)

<!-- mdformat-toc end -->

## From 1.0.x to 1.1.x

When upgrading from `v1.0.X` to `v1.1.X`, a minor modification will need to be
made to the Slurm Helm Chart's values file. The field `jwtHs256KeyRef` was
refactored to the map `jwtKey` for the sake of simplicity and to enable
conditional secret creation and annotation.

The field `jwtHs256KeyRef` must be replaced with the field `jwtKey`.
`jwtKey.create` should be set to false, to prevent slurm-operator from
attempting to create a new secret for the deployment. `jwtKey.secretRef` should
be modified to reference the existing secret in your environment.

For example:

```yaml
jwtKey:
  create: false
  secretRef:
    name: slurm-auth-jwths256
    key: jwt_hs256.key
```

## From 1.1.x to 1.2.x

### Shared defaults for loginsets and nodesets

Every entry under `loginsets:` and `nodesets:` now inherits from a shared
`_defaults:` block, so custom entries no longer need to redeclare every
field. Existing values files continue to work unchanged — `slinky` overrides
still land on top of `_defaults`.

The fields that previously lived under `loginsets.slinky.*` and
`nodesets.slinky.*` in the chart's own `values.yaml` have moved to
`loginsets._defaults.*` and `nodesets._defaults.*`. The `slinky:` entry is
now a thin stub. If you are diffing the chart's `values.yaml` across
versions, look for your defaults there.

To apply a chart-wide override (e.g. pin every loginset to an internal
image registry), set the field on `_defaults` instead of duplicating it
across every entry:

```yaml
loginsets:
  _defaults:
    login:
      image:
        repository: registry.internal.example.com/slinky/login
```

Refs: [issue #176](https://github.com/SlinkyProject/slurm-operator/issues/176)
