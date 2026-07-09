#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SLURM_DIR="/etc/slurm"
INTERVAL="5"

function getHash() {
	# Ignore kubelet's hidden projected-volume generation directories (..data, ..20xx_*)
	# and hash only the stable top-level logical config files.
	find "$SLURM_DIR" -maxdepth 1 \( -type f -o -type l \) ! -name '..*' -exec sha256sum {} \; |
		sort -k2 |
		sha256sum
}

function reconfigure() {
	# Issue cluster reconfigure request
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
	lastHash="$(getHash)"
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
