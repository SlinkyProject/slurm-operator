# Slinky Release Versioning

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Slinky Release Versioning](#slinky-release-versioning)
  - [Table of Contents](#table-of-contents)
  - [Releases](#releases)
    - [Schema](#schema)
    - [Major](#major)

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
  - `X.Y.0` occur after `X.(Y-1).0`.
- `X.Y.Z`, `Z > 0` (Branch: `release-X.Y`)
  - Patch releases are released as we cherry-pick commits into the `release-X.Y`
    branch, as needed.
  - `X.Y.Z` is cut straight from the `release-X.Y` branch.

### Major

There is no mandated timeline for major versions and there are currently no
criteria for shipping `v2.0.0`. We have not so far applied a rigorous
interpretation of semantic versioning with respect to incompatible changes of
any kind (e.g., component flag changes).

<!-- Links -->

[containers]: https://github.com/SlinkyProject/containers
[semver]: https://semver.org/
[slurm-bridge]: https://github.com/SlinkyProject/slurm-bridge
[slurm-client]: https://github.com/SlinkyProject/slurm-client
[slurm-operator]: https://github.com/SlinkyProject/slurm-operator
