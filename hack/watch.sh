#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

TIMEOUT="${TIMEOUT:-5s}"

function section() {
	local title="$1" top="$2"
	shift 2
	local out rc=0
	out="$("$@" 2>&1)" || rc=$?
	if ((rc == 0)); then
		local total
		total=$(printf '%s\n' "$out" | wc -l)
		total=$((total > 0 ? total - 1 : 0))
		echo "===== $title (TOP $top) [Total: $total] ====="
		printf '%s\n' "$out" | head -n $((top + 1))
	else
		echo "===== $title ====="
		echo "Error: $out"
	fi
	echo
}

function monitor_cluster() {
	echo "DATE: $(date --rfc-3339=seconds)"
	echo

	section "OPERATOR CHART" 5 \
		kubectl -n slinky get pods -l app.kubernetes.io/part-of=slurm-operator --request-timeout="$TIMEOUT"

	section "SLURM CHART" 10 \
		kubectl -n slurm get pods -l app.kubernetes.io/part-of=slurm --request-timeout="$TIMEOUT"

	section "NODESET STATUS" 5 \
		kubectl -n slurm get -o wide nodesets.slinky.slurm.net --request-timeout="$TIMEOUT"

	local name active_controller
	name=$(kubectl -n slurm get pods -l controller.slinky.slurm.net/active=true \
		-o jsonpath='{.items[0].metadata.name}' --request-timeout="$TIMEOUT" 2>/dev/null || true)
	if [[ -n $name ]]; then
		active_controller="pods/$name"
	else
		active_controller="statefulsets/slurm-controller"
	fi
	echo "CONTROLLER: $active_controller"
	echo

	section "SINFO" 10 \
		kubectl -n slurm exec "$active_controller" --request-timeout="$TIMEOUT" -- sinfo

	section "SQUEUE" 10 \
		kubectl -n slurm exec "$active_controller" --request-timeout="$TIMEOUT" -- squeue
}

if [[ ${1:-} == "--once" ]]; then
	monitor_cluster
	exit 0
fi

INTERVAL="${1:-2}"
exec watch --no-title --color --interval="$INTERVAL" "$0" --once
