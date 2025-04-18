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
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Common imagePullPolicy
*/}}
{{- define "slurm.imagePullPolicy" -}}
{{ .Values.imagePullPolicy | default "IfNotPresent" }}
{{- end }}

{{/*
Common imagePullSecrets
*/}}
{{- define "slurm.imagePullSecrets" -}}
{{- with .Values.imagePullSecrets -}}
imagePullSecrets:
  {{- . | toYaml | nindent 2 }}
{{- end }}
{{- end }}

{{/*
Determine slurm image repository
*/}}
{{- define "slurm.image.repository" -}}
{{- print "ghcr.io/slinkyproject" -}}
{{- end }}

{{/*
Define image tag
*/}}
{{- define "slurm.image.tag" -}}
{{- printf "%s-ubuntu24.04" .Chart.AppVersion -}}
{{- end }}


{{- /* Helper function for power calculation */}}
{{- define "pow" -}}
    {{- $base := index . 0 -}}
    {{- $exp := index . 1 -}}
    {{- $result := 1 -}}
    {{- range $i := until $exp -}}
        {{- $result = mul $result $base -}}
    {{- end -}}
    {{- $result -}}
{{- end -}}

{{/*
Convert Kubernetes storage request to Megabytes
Supports all resource units that Kubernetes supports
*/}}
{{- define "slurm.convertToMB" -}}
{{- $input := . -}}
{{- $value := $input -}}
{{- $multiplier := 1 -}}
{{- $isBinary := false -}}

{{- /* Handle bytes */}}
{{- if eq (kindOf $input) "float64" }}
    {{- $value = $input }}

{{- /* Handle decimal suffixes */}}
{{- else if hasSuffix "E" $input }}
    {{- $value = trimSuffix "E" $input }}
    {{- $multiplier = include "pow" (list 1000 6) }}
{{- else if hasSuffix "P" $input }}
    {{- $value = trimSuffix "P" $input }}
    {{- $multiplier = include "pow" (list 1000 5) }}
{{- else if hasSuffix "T" $input }}
    {{- $value = trimSuffix "T" $input }}
    {{- $multiplier = include "pow" (list 1000 4) }}
{{- else if hasSuffix "G" $input }}
    {{- $value = trimSuffix "G" $input }}
    {{- $multiplier = include "pow" (list 1000 3) }}
{{- else if hasSuffix "M" $input }}
    {{- $value = trimSuffix "M" $input }}
    {{- $multiplier = include "pow" (list 1000 2) }}
{{- else if hasSuffix "k" $input }}
    {{- $value = trimSuffix "k" $input }}
    {{- $multiplier = include "pow" (list 1000 1) }}

{{- /* Handle binary suffixes */}}
{{- else if hasSuffix "Ei" $input }}
    {{- $value = trimSuffix "Ei" $input }}
    {{- $multiplier = include "pow" (list 1024 6) }}
    {{- $isBinary = true }}
{{- else if hasSuffix "Pi" $input }}
    {{- $value = trimSuffix "Pi" $input }}
    {{- $multiplier = include "pow" (list 1024 5) }}
    {{- $isBinary = true }}
{{- else if hasSuffix "Ti" $input }}
    {{- $value = trimSuffix "Ti" $input }}
    {{- $multiplier = include "pow" (list 1024 4) }}
    {{- $isBinary = true }}
{{- else if hasSuffix "Gi" $input }}
    {{- $value = trimSuffix "Gi" $input }}
    {{- $multiplier = include "pow" (list 1024 3) }}
    {{- $isBinary = true }}
{{- else if hasSuffix "Mi" $input }}
    {{- $value = trimSuffix "Mi" $input }}
    {{- $multiplier = include "pow" (list 1024 2) }}
    {{- $isBinary = true }}
{{- else if hasSuffix "Ki" $input }}
    {{- $value = trimSuffix "Ki" $input }}
    {{- $multiplier = include "pow" (list 1024 1) }}
    {{- $isBinary = true }}
{{- end -}}

{{- /* Convert to numeric and calculate MB */}}
{{- $numericValue := float64 $value }}
{{- $bytes := mul $numericValue (int64 $multiplier) }}
{{- if $isBinary }}
    {{- div $bytes (mul 1024 1024) -}}
{{- else }}
    {{- div $bytes (mul 1000 1000) -}}
{{- end -}}
{{- end -}}
