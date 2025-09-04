{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-FileCopyrightText: Copyright (C) NVIDIA CORPORATION & AFFILIATES. All rights reserved.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Check if DCGM integration is enabled
*/}}
{{- define "slurm.dcgm.enabled" -}}
{{- .Values.vendor.nvidia.dcgm.enabled -}}
{{- end }}

{{/*
Get the DCGM job mapping directory
*/}}
{{- define "slurm.dcgm.jobMappingDir" -}}
{{- .Values.vendor.nvidia.dcgm.jobMappingDir | default "/var/lib/dcgm-exporter/job-mapping" -}}
{{- end }}

{{/*
Check if a nodeset has GPU resources allocated
*/}}
{{- define "slurm.dcgm.nodesetHasGPU" -}}
{{- $hasGPU := false -}}
{{- with .resources -}}
  {{- with .limits -}}
    {{- if index . "nvidia.com/gpu" -}}
      {{- $hasGPU = true -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- print $hasGPU -}}
{{- end }}

{{/*
Generate DCGM prolog script content
*/}}
{{- define "slurm.dcgm.prologScript" -}}
{{- $jobMappingDir := include "slurm.dcgm.jobMappingDir" . -}}
{{- .Files.Get "_vendor/nvidia/dcgm/scripts/90-prolog-dcgm.sh" | replace "__JOB_MAPPING_DIR__" $jobMappingDir -}}
{{- end }}

{{/*
Generate DCGM epilog script content
*/}}
{{- define "slurm.dcgm.epilogScript" -}}
{{- $jobMappingDir := include "slurm.dcgm.jobMappingDir" . -}}
{{- .Files.Get "_vendor/nvidia/dcgm/scripts/90-epilog-dcgm.sh" | replace "__JOB_MAPPING_DIR__" $jobMappingDir -}}
{{- end }}
