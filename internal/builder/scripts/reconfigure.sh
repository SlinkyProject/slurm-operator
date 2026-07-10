#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SLURM_DIR="/etc/slurm"
INTERVAL="5"

# Returns the canonical checksum hash of all Slurm configuration files.
function getHash() {
	echo "$(find "$SLURM_DIR" -type f -exec sha256sum {} \; | sort -k2 | sha256sum)"
}

# Issues a cluster reconfigure request with retry.
function reconfigure() {
	echo "[$(date)] Reconfiguring Slurm..."
	until scontrol reconfigure; do
		echo "[$(date)] Failed to reconfigure, try again..."
		sleep 2
	done
	echo "[$(date)] SUCCESS"
}

function main() {
	local lastHash=""
	local newHash=""

	echo "[$(date)] Start '$SLURM_DIR' polling"
	while true; do
		newHash="$(getHash)"
		if [ "$newHash" != "$lastHash" ]; then
			reconfigure
			lastHash="$newHash"
		fi
		sleep "$INTERVAL"
	done
}
main
