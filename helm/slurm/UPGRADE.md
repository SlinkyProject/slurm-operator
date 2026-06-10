# Upgrade

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Upgrade](#upgrade)
  - [Table of Contents](#table-of-contents)
  - [From 1.1.x to 1.2.x](#from-11x-to-12x)
  - [From 1.0.x to 1.1.x](#from-10x-to-11x)

<!-- mdformat-toc end -->

## From 1.1.x to 1.2.x

The helm chart interface has changed. Please review the following changes and
the migration steps.

- Value `nodesets.slinky` is no longer enabled by default. To keep this named
  NodeSet, install with `--set-json 'nodesets={"slinky":{}}'`.
- Value `partitions.all` is no longer enabled by default. To enable it, install
  with `--set partitions.all.enabled=true`.

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
