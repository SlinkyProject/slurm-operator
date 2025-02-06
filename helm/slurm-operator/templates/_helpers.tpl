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
Common operator labels
*/}}
{{- define "slurm-operator.operator.labels" -}}
helm.sh/chart: {{ include "slurm-operator.chart" . }}
{{ include "slurm-operator.operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector operator labels
*/}}
{{- define "slurm-operator.operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "slurm-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the operator service account to use
*/}}
{{- define "slurm-operator.operator.serviceAccountName" -}}
{{- if .Values.operator.serviceAccount.create }}
{{- default (include "slurm-operator.fullname" .) .Values.operator.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.operator.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Determine operator image repository
*/}}
{{- define "slurm-operator.image.repository" -}}
{{ .Values.image.repository | default "slinky.slurm.net/slurm-operator" }}
{{- end }}

{{/*
Define operator image tag
*/}}
{{- define "slurm-operator.image.tag" -}}
{{ .Values.image.tag | default .Chart.Version }}
{{- end }}

{{/*
Determine operator image reference (repo:tag)
*/}}
{{- define "slurm-operator.imageRef" -}}
{{ printf "%s:%s" (include "slurm-operator.image.repository" .) (include "slurm-operator.image.tag" .) | quote }}
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
