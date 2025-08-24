#!/bin/bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-FileCopyrightText: Copyright (C) NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# DCGM GPU-to-Job Mapping Prolog Script
# 
# This script runs when jobs start on GPU nodes and creates job mapping files
# for the DCGM exporter to correlate GPU metrics with Slurm job IDs.

function log_msg() {
  echo "[$(date)] [$$] $*" >&2
}

function set_globals() {
  declare -g METRICS_DIR="__JOB_MAPPING_DIR__"
}

function make_gpu_job_files() {
  if [[ -z ${CUDA_VISIBLE_DEVICES:-} ]]; then
    log_msg "no gres cuda devices requested by user"
    return
  fi

  if [[ ! -d "${METRICS_DIR:-}" ]]; then
    mkdir -p "${METRICS_DIR:-}" || (log_msg "unable to create the data dir ${METRICS_DIR:-missing}"; return)
  fi

  mapfile -t -d ',' cuda_devs<<<"${CUDA_VISIBLE_DEVICES:-}"
  cuda_devs[-1]="${cuda_devs[-1]%$'\n'}"

  for gpu_id in "${cuda_devs[@]}"; do
    log_msg "writing ${METRICS_DIR:-}/${gpu_id:-99}"
    printf "%s" "${SLURM_JOB_ID:-0}" > "${METRICS_DIR:-}/${gpu_id:-99}" || log_msg "unable to write job file"
  done
}

set -o nounset -o errexit -o pipefail -o errtrace
set_globals
make_gpu_job_files
