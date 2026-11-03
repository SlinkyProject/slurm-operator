# Slinky Release Versioning

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Slinky Release Versioning](#slinky-release-versioning)
  - [Table of Contents](#table-of-contents)
  - [Releases](#releases)
    - [Schema](#schema)
    - [Major](#major)
  - [CRD Versions](#crd-versions)
  - [Helm Chart Versions](#helm-chart-versions)

<!-- mdformat-toc end -->

## Releases

**X.Y.Z** refers to the version (git tag) of Slinky that is released. (**X** is
the major version, **Y** is the minor version, and **Z** is the patch version,
following [Semantic Versioning][semver] terminology.)

All Slinky components (e.g. [slurm-operator], [slurm-bridge], [slurm-client],
and their helm charts and images) are versioned in lock-step. There may be skew
between the actual day they are tagged and released due to dependency chains and
CI.

Images derived from Slinky [containers] are versioned and released separately.
These container images are versioned in accordance with the application they
contain. Hence Slurm daemon images are versioned in alignment with Slurm proper.

### Schema

- `X.Y.Z-rcW` (Branch: `release-X.Y`)
  - When `main` is feature-complete for `X.Y`, we may cut the `release-X.Y`
    branch prior to the desired `X.Y.0` date and cherrypick only PRs essential
    to `X.Y`.
  - This cut will be marked as `X.Y.0-rc0`, and `main` will be revved to
    `X.(Y+1).0-rc0`.
  - If we're not satisfied with `X.Y.0-rc0`, we'll release other rc releases,
    (`X.Y.0-rcW` where `W > 0`) as necessary.
- `X.Y.0` (Branch: `release-X.Y`)
  - Final release, cut from the `release-X.Y` branch.
  - `X.Y.0` occurs after `X.(Y-1).0`.
- `X.Y.Z`, `Z > 0` (Branch: `release-X.Y`)
  - Patch releases are released as we cherry-pick commits into the `release-X.Y`
    branch, as needed.
  - `X.Y.Z` is cut straight from the `release-X.Y` branch.

### Major

There is no mandated timeline for major versions and there are currently no
criteria for shipping `v2.0.0`. We have not so far applied a rigorous
interpretation of semantic versioning with respect to incompatible changes of
any kind (e.g., component flag changes).

## CRD Versions

[CRDs] have their own [versioning][crd-versioning] (e.g. `v1alpha1`, `v1alpha2`,
`v1beta1`). The Slinky version does not strongly correlate with the CRD version.

New CRD versions should be completely backwards compatible with old CRD
versions; old CRD versions will automatically be converted to the new CRD
version (if applicable). Therefore, if `v1beta2` was the latest installed CRD
version, then resources that are installed as `v1beta1` will still work.

Slinky `v1.Y` releases may introduce new fields to existing CRD versions and
deprecate certain fields. Only a new CRD version can safely remove deprecated
fields or restructure fields.

When installing CR resources, it is recommended to use the latest installed CRD
version. However, older CRD versions should work but may be missing new fields
which control new functionality.

## Helm Chart Versions

Helm charts share their version with the Slinky release version.

Slinky `v1.Y` releases may introduce structural changes to the `values.yaml`
such that upgrading Slinky release series (e.g. `v1.0.Z` => `v1.1.Z`) of a chart
may need extra attention.

<!-- Links -->

[containers]: https://github.com/SlinkyProject/containers
[crd-versioning]: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/
[crds]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions
[semver]: https://semver.org/
[slurm-bridge]: https://github.com/SlinkyProject/slurm-bridge
[slurm-client]: https://github.com/SlinkyProject/slurm-client
[slurm-operator]: https://github.com/SlinkyProject/slurm-operator
