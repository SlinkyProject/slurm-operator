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
