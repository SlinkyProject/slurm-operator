#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# Assume env contains:
# SLURM_USER - username or UID

SLURM_MOUNT=/mnt/slurm
SLURM_DIR=/mnt/etc/slurm
INTERVAL=30
INIT_RECONFIGURE=false

function reconfigure() {
	local rsync_cmd='rsync -vaLrzPci --delete --include="*.conf" --include="prolog-*" --include="epilog-*" --exclude="*" "${SLURM_MOUNT}/" "${SLURM_DIR}"'

	if [ -z "$(eval "$rsync_cmd --dry-run | grep '\./'")" ] && $INIT_RECONFIGURE; then
		return
	fi

	# Sync Slurm config files, ignore all other files
	eval "$rsync_cmd"
	find "${SLURM_DIR}" -type f -name "*.conf" -print0 | xargs -0r chown -v "${SLURM_USER}:${SLURM_USER}"
	find "${SLURM_DIR}" -type f -name "*.conf" -print0 | xargs -0r chmod -v 644
	find "${SLURM_DIR}" -type f -name "*.yaml" -print0 | xargs -0r chown -v "${SLURM_USER}:${SLURM_USER}"
	find "${SLURM_DIR}" -type f -name "*.yaml" -print0 | xargs -0r chmod -v 644
	find "${SLURM_DIR}" -type f -regextype posix-extended -regex "^.*/(pro|epi)log-.*$" -print0 | xargs -0r chown -v "${SLURM_USER}:${SLURM_USER}"
	find "${SLURM_DIR}" -type f -regextype posix-extended -regex "^.*/(pro|epi)log-.*$" -print0 | xargs -0r chmod -v 755

	# Config files are not in expected directory `/etc/slurm`
	export SLURM_CONF="$SLURM_MOUNT/slurm.conf"

	# Issue cluster reconfigure request
	echo "[$(date)] Reconfiguring Slurm..."
	until scontrol reconfigure; do
		echo "[$(date)] Failed to reconfigure, try again..."
		sleep 2
	done
	echo "[$(date)] DONE"
	INIT_RECONFIGURE=true
}

function main() {
	echo "[$(date)] Start Slurm config change polling"
	while true; do
		reconfigure
		sleep "$INTERVAL"
	done
}
main
