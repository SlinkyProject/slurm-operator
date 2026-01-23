#!/usr/bin/env sh
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

set -eu

SLURM_USER_UID="401"
SLURM_USER_GID="401"
SLURM_MOUNT=/mnt/slurm
SLURM_DIR=/mnt/etc/slurm

# Workaround to ephemeral volumes not supporting securityContext
# https://github.com/kubernetes/kubernetes/issues/81089

# Copy Slurm config files, secrets, and scripts
mkdir -p "$SLURM_DIR"
find "${SLURM_MOUNT}" -type f -name "*.conf" -print0 | xargs -0r cp -vt "${SLURM_DIR}"
find "${SLURM_MOUNT}" -type f -name "*.key" -print0 | xargs -0r cp -vt "${SLURM_DIR}"

# Set general permissions and ownership
find "${SLURM_DIR}" -type f -print0 | xargs -0r chown -v "${SLURM_USER_UID}:${SLURM_USER_GID}"
find "${SLURM_DIR}" -type f -name "*.conf" -print0 | xargs -0r chmod -v 644
find "${SLURM_DIR}" -type f -name "slurmdbd.conf" -print0 | xargs -0r chmod -v 600
find "${SLURM_DIR}" -type f -name "*.key" -print0 | xargs -0r chmod -v 600
find "${SLURM_DIR}" -type f -name "*.key" -print0 | xargs -0r chown -v "${SLURM_USER_UID}:${SLURM_USER_GID}"

# Display Slurm directory files
ls -lAF "${SLURM_DIR}"
