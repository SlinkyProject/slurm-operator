{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-FileCopyrightText: Copyright (C) NVIDIA CORPORATION & AFFILIATES. All rights reserved.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Check if DCGM integration is enabled
*/}}
{{- define "vendor.dcgm.enabled" -}}
{{- .Values.vendor.nvidia.dcgm.enabled | ternary "true" "" -}}
{{- end }}

{{/*
Get the DCGM job mapping directory
*/}}
{{- define "vendor.dcgm.jobMappingDir" -}}
{{- .Values.vendor.nvidia.dcgm.jobMappingDir | default "/var/lib/dcgm-exporter/job-mapping" -}}
{{- end }}

{{/*
Check if a nodeset has GPU resources allocated
*/}}
{{- define "vendor.dcgm.nodesetHasGPU" -}}
{{- $hasGPU := "" -}}
{{- with .resources -}}
  {{- with .limits -}}
    {{- if index . "nvidia.com/gpu" -}}
      {{- $hasGPU = "nvidia.com/gpu" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- print $hasGPU -}}
{{- end }}

{{/*
Generate DCGM prolog configmap name.
*/}}
{{- define "vendor.dcgm.prologName" -}}
{{- printf "%s-prolog-dcgm" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Generate DCGM epilog configmap name.
*/}}
{{- define "vendor.dcgm.epilogName" -}}
{{- printf "%s-epilog-dcgm" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Generate DCGM prolog script content
*/}}
{{- define "vendor.dcgm.prologScripts" -}}
{{- $scriptPriority := .Values.vendor.nvidia.dcgm.scriptPriority | default "90" }}
{{- $jobMappingDir := include "vendor.dcgm.jobMappingDir" . -}}
{{- range $path, $_ := .Files.Glob "_vendor/nvidia/dcgm/scripts/prolog/*.sh" -}}
  {{- $contents := $.Files.Get $path | replace "__JOB_MAPPING_DIR__" $jobMappingDir -}}
  {{- printf "prolog-%s-%s" $scriptPriority (base $path) | nindent 0 -}}: |
    {{- $contents | nindent 2 -}}
{{- end }}
{{- end }}

{{/*
Generate DCGM epilog script content
*/}}
{{- define "vendor.dcgm.epilogScripts" -}}
{{- $scriptPriority := .Values.vendor.nvidia.dcgm.scriptPriority | default "90" }}
{{- $jobMappingDir := include "vendor.dcgm.jobMappingDir" . -}}
{{- range $path, $_ := .Files.Glob "_vendor/nvidia/dcgm/scripts/epilog/*.sh" -}}
  {{- $contents := $.Files.Get $path | replace "__JOB_MAPPING_DIR__" $jobMappingDir -}}
  {{- printf "epilog-%s-%s" $scriptPriority (base $path) | nindent 0 -}}: |
    {{- $contents | nindent 2 -}}
{{- end }}
{{- end }}
