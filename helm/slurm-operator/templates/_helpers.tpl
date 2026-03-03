{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "slurm-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "slurm-operator.fullname" -}}
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
{{- define "slurm-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Allow the release namespace to be overridden
*/}}
{{- define "slurm-operator.namespace" -}}
{{ default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Common imagePullPolicy
*/}}
{{- define "slurm-operator.imagePullPolicy" -}}
{{ .Values.imagePullPolicy | default "IfNotPresent" }}
{{- end }}

{{/*
Common imagePullSecrets
*/}}
{{- define "slurm-operator.imagePullSecrets" -}}
{{- with .Values.imagePullSecrets -}}
imagePullSecrets:
  {{- . | toYaml | nindent 2 }}
{{- end }}
{{- end }}

{{/*
Define the API group
*/}}
{{- define "slurm-operator.apiGroup" -}}
{{- print "slinky.slurm.net" }}
{{- end }}
