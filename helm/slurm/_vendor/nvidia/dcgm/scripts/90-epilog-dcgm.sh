#!/bin/bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-FileCopyrightText: Copyright (C) NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# DCGM GPU-to-Job Mapping Epilog Script
# 
# This script runs when jobs complete on GPU nodes and cleans up job mapping files
# to prevent stale job mappings in the DCGM exporter.

function log_msg() {
  echo "[$(date)] [$$] $*" >&2
}

function set_globals() {
  declare -g METRICS_DIR="__JOB_MAPPING_DIR__"
}

function clean_gpu_job_files() {
  if [[ -z ${CUDA_VISIBLE_DEVICES:-} ]]; then
    log_msg "no gres cuda devices requested by user"
    return
  fi

  mapfile -t -d ',' cuda_devs<<<"${CUDA_VISIBLE_DEVICES:-}"
  cuda_devs[-1]="${cuda_devs[-1]%$'\n'}"

  for gpu_id in "${cuda_devs[@]}"; do
    log_msg "removing ${METRICS_DIR:-}/${gpu_id:-99}"
    rm -f "${METRICS_DIR:-}/${gpu_id:-99}" || log_msg "unable to remove file ${METRICS_DIR:-}/${gpu_id:-99}"
  done
}

set -o nounset -o errexit -o pipefail -o errtrace
set_globals
clean_gpu_job_files
