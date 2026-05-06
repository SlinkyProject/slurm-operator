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

### Signing Your Work

- We require that all contributors "sign-off" on their commits. This certifies
  that the contribution is your original work, or you have rights to submit it
  under the same license, or a compatible license.
  - Any contribution which contains commits that are not Signed-Off will not be
    accepted.
- To sign off on a commit you simply use the `--signoff` (or `-s`) option when
  committing your changes:
  ```bash
  $ git commit -s -m "Add cool feature."
  ```
  This will append the following to your commit message:
  ```
  Signed-off-by: Your Name <your@email.com>
  ```
- Full text of the DCO (https://developercertificate.org/):
  ```
    Developer Certificate of Origin
    Version 1.1
    Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
    Everyone is permitted to copy and distribute verbatim copies of this
    license document, but changing it is not allowed.
    Developer's Certificate of Origin 1.1
    By making a contribution to this project, I certify that:
    (a) The contribution was created in whole or in part by me and I
        have the right to submit it under the open source license
        indicated in the file; or
    (b) The contribution is based upon previous work that, to the best
        of my knowledge, is covered under an appropriate open source
        license and I have the right under that license to submit that
        work with modifications, whether created in whole or in part
        by me, under the same open source license (unless I am
        permitted to submit under a different license), as indicated
        in the file; or
    (c) The contribution was provided directly to me by some other
        person who certified (a), (b) or (c) and I have not modified
        it.
    (d) I understand and agree that this project and the contribution
        are public and that a record of the contribution (including all
        personal information I submit with it, including my sign-off) is
        maintained indefinitely and may be redistributed consistent with
        this project or the open source license(s) involved.

  ```

### Community standards

- [Code of Conduct](https://github.com/SlinkyProject/slurm-operator/blob/main/CODE_OF_CONDUCT.md)
- [Security policy](https://github.com/SlinkyProject/slurm-operator/blob/main/SECURITY.md)

### Attribution

Project contributing guidelines are based on common open-source practice and the
[PLC-OSS-Template](https://github.com/NVIDIA-GitHub-Management/PLC-OSS-Template)
baseline used for NVIDIA OSS projects.
