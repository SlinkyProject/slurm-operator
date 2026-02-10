{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "slurm.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "slurm.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "slurm.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Allow the release namespace to be overridden
*/}}
{{- define "slurm.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "slurm.labels" -}}
helm.sh/chart: {{ include "slurm.chart" . }}
app.kubernetes.io/part-of: slurm
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Format image reference from image object.
*/}}
{{- define "format-image" -}}
{{- if kindIs "string" . -}}
  {{- $image := required "image is required" . -}}
  {{- printf $image | toString -}}
{{- else -}}
  {{- $repository := required "image repository is required" .repository -}}
  {{- $tag := required "image tag is required" .tag -}}
  {{- printf "%s:%s" $repository $tag | toString -}}
{{- end -}}
{{- end -}}

{{/*
Format container object.
*/}}
{{- define "format-container" -}}
{{- $container := omit . "image" -}}
{{- $_ := set $container "image" (include "format-image" .image) -}}
{{ toYaml $container }}
{{- end -}}

{{/*
Format pod template object.
*/}}
{{- define "format-podTemplate" -}}
{{- with . -}}
template:
  {{- include "format-metadata" . | nindent 2 -}}
  {{- include "format-podSpec" . | nindent 2 -}}
{{- end -}}
{{- end -}}

{{/*
Format pod metadata object.
*/}}
{{- define "format-metadata" -}}
{{- with .metadata -}}
metadata:
  {{- toYaml . | nindent 2 }}
{{- end -}}
{{- end -}}

{{/*
Format pod spec object.
*/}}
{{- define "format-podSpec" -}}
{{- with .spec -}}
spec:
  {{- toYaml . | nindent 2 }}
{{- end -}}
{{- end -}}

{{/*
Converts a list to a key value CSV.
Ref: https://github.com/helm/helm/issues/9379
*/}}
{{- define "_toList" -}}
{{- $items := list -}}
{{- range $key, $val := . -}}
  {{- if $val -}}
    {{- $items = append $items (printf "%s=%s" $key (join "," $val)) -}}
  {{- end -}}
{{- end -}}
{{- join ";" $items -}}
{{- end -}}

{{/*
Parse resources object and convert units.
Ref: https://github.com/helm/helm/issues/11376#issuecomment-1256831105
*/}}
{{- define "resource-quantity" -}}
{{- $value := . -}}
{{- $unit := 1.0 -}}
{{- if typeIs "string" . -}}
  {{- $base2 := dict "Ki" 0x1p10 "Mi" 0x1p20 "Gi" 0x1p30 "Ti" 0x1p40 "Pi" 0x1p50 "Ei" 0x1p60 -}}
  {{- $base10 := dict "m" 1e-3 "k" 1e3 "M" 1e6 "G" 1e9 "T" 1e12 "P" 1e15 "E" 1e18 -}}
  {{- range $k, $v := merge $base2 $base10 -}}
    {{- if hasSuffix $k $ -}}
      {{- $value = trimSuffix $k $ -}}
      {{- $unit = $v -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- mulf (float64 $value) $unit -}}
{{- end -}}

{{/*
ToYaml dict after handling storageClassName.
Ref: https://github.com/helm/helm/issues/2600
*/}}
{{- define "toYaml-set-storageClassName" -}}
{{- $out := . | default dict -}}
{{ $storageClassName := dig "storageClassName" nil $out }}
{{- if or (eq $storageClassName "") (eq $storageClassName "-") -}}
  {{- $_ := set $out "storageClassName" "" -}}
{{- else if $storageClassName }}
  {{- $_ := set $out "storageClassName" $storageClassName -}}
{{- else }}
  {{- $out = omit $out "storageClassName" -}}
{{- end }}
{{- toYaml $out -}}
{{- end -}}

{{/*
Allow the PriorityClassName to be overridden.
*/}}
{{- define "slurm.priorityClassName" -}}
{{- default "slurm-priority-critical" .Values.priorityClass.name }}
{{- end }}
