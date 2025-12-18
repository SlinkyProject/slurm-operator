{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Define worker name
*/}}
{{- define "slurm.worker.name" -}}
{{- printf "%s-worker" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Define worker port
*/}}
{{- define "slurm.worker.port" -}}
{{- print "6818" -}}
{{- end }}

{{/*
Determine worker extraConf (e.g. `--conf <extraConf>`)
*/}}
{{- define "slurm.worker.extraConf" -}}
{{- $extraConf := list -}}
{{- if .extraConf -}}
  {{- $extraConf = splitList " " .extraConf -}}
{{- else if .extraConfMap -}}
  {{- $extraConf = (include "_toList" .extraConfMap) | splitList ";" -}}
{{- end }}
{{- join " " $extraConf -}}
{{- end }}

{{/*
Determine worker partition config
*/}}
{{- define "slurm.worker.partitionConfig" -}}
{{- $config := list -}}
{{- if .config -}}
  {{- $config = list .config -}}
{{- else if .configMap -}}
  {{- $config = (include "_toList" .configMap) | splitList ";" -}}
{{- end }}
{{- join " " $config -}}
{{- end }}

{{/*
Returns the parsed resource limits for POD_CPUS.
*/}}
{{- define "slurm.worker.podCpus" -}}
{{- $out := 0 -}}
{{- with .resources }}{{- with .limits }}{{- with .cpu }}
  {{- $out = include "resource-quantity" . | float64 | ceil | int -}}
{{- end }}{{- end }}{{- end }}
{{- print $out -}}
{{- end -}}

{{/*
Returns the parsed resource limits for POD_MEMORY, in Mebibytes (MiB).*/}}
{{- define "slurm.worker.podMemory" -}}
{{- $out := 0 -}}
{{- with .resources }}{{- with .limits }}{{- with .memory }}
  {{- $mebibytes := (include "resource-quantity" "1Mi") | float64 -}}
  {{- $out = divf (include "resource-quantity" . | float64) $mebibytes | ceil | int -}}
{{- end }}{{- end }}{{- end }}
{{- print $out -}}
{{- end -}}
