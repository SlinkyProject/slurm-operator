#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

INTERVAL="${1:-2}"

watch --no-title --color --interval="$INTERVAL" bash -c 'cat <<-EOF
DATE: $(date --rfc-3339=seconds)

===== OPERATOR CHART (TOP 5) [Total: $(kubectl -n slinky get --no-headers pods -l app.kubernetes.io/part-of=slurm-operator 2>/dev/null | wc -l)] =====
$(kubectl -n slinky get pods -l app.kubernetes.io/part-of=slurm-operator 2>&1)

===== SLURM CHART (TOP 10) [Total: $(kubectl -n slurm get --no-headers pods -l app.kubernetes.io/part-of=slurm 2>/dev/null | wc -l)] =====
$(kubectl -n slurm get pods -l app.kubernetes.io/part-of=slurm 2>&1 | head -n 11)

===== NODESET STATUS (TOP 5) [Total: $(kubectl -n slurm get --no-headers nodesets.slinky.slurm.net 2>/dev/null | wc -l)] =====
$(kubectl -n slurm get -o wide nodesets.slinky.slurm.net 2>&1 | head -n 6)

===== SINFO (TOP 10) [Total: $(kubectl -n slurm exec statefulsets/slurm-controller -- sinfo --noheader 2>/dev/null | wc -l)] =====
$(kubectl -n slurm exec statefulsets/slurm-controller -- sinfo 2>&1 | head -n 11)

===== SQUEUE (TOP 10) [Total: $(kubectl -n slurm exec statefulsets/slurm-controller -- squeue --noheader 2>/dev/null | wc -l)] =====
$(kubectl -n slurm exec statefulsets/slurm-controller -- squeue 2>&1 | head -n 11)
EOF'
