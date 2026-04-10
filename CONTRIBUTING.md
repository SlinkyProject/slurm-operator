<!-- SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved. -->

# Contributing to Slurm Operator

Thank you for your interest in contributing to Slurm Operator.

## Ways to contribute

1. **Report a bug, request a feature, or suggest documentation changes** Open an
   issue in this project:
   [https://github.com/SlinkyProject/slurm-operator/issues](https://github.com/SlinkyProject/slurm-operator/issues).
   For bugs, include relevant environment and version details in the issue so we
   can reproduce the problem.

1. **Propose a larger feature** Open an issue to describe the problem and
   proposed approach before investing significant implementation time, so
   maintainers can provide feedback on direction and design.

1. **Submit a fix or feature as a pull request** Follow the
   [Code contributions](#code-contributions) section below.

Issues and pull requests in this repository are handled on a **best-effort**
basis. For production support, see
[SchedMD Support](https://support.schedmd.com/).

## Code contributions

### Development setup

Read
[`README.md`](https://github.com/SlinkyProject/slurm-operator/blob/main/README.md)
for clone, build, and test instructions for this repository.

### Pull requests

1. Fork or branch from `main` (unless a maintainer directs you otherwise).
1. **Pre-commit:** With [pre-commit](https://pre-commit.com/) installed on your
   machine, run `pre-commit install` from the repository root to register Git
   hooks, then run `pre-commit install --install-hooks` once to initialize hook
   environments. Do this before you commit so local checks match CI.
1. Keep changes focused and include tests where appropriate.
1. Update documentation if you change user-visible behavior.
1. Open a pull request against this repository. Link related issues.
1. Ensure CI passes; address review feedback.

### Developer Certificate of Origin / sign-off

This project is released under the Apache License 2.0. By contributing, you
agree that your contributions are licensed under that license. Use signed
commits or include a `Signed-off-by` line in your commits as described in the
[Git documentation](https://git-scm.com/docs/git-commit#Documentation/git-commit.txt--s).

### Community standards

- [Code of Conduct](https://github.com/SlinkyProject/slurm-operator/blob/main/CODE_OF_CONDUCT.md)
- [Security policy](https://github.com/SlinkyProject/slurm-operator/blob/main/SECURITY.md)

### Attribution

Project contributing guidelines are based on common open-source practice and the
[PLC-OSS-Template](https://github.com/NVIDIA-GitHub-Management/PLC-OSS-Template)
baseline used for NVIDIA OSS projects.
