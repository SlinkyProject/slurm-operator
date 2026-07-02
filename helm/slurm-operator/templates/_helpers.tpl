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
Validate a metrics port value and return it as a canonical decimal number.
A value of 0 (or unset) disables the metrics server. Rejects anything that
is not a plain decimal integer between 0 and 65535, including leading-zero
strings, which strconv would otherwise read as octal or truncate to 0.
Note that unquoted leading-zero literals (e.g. 0777) are resolved to their
octal value by YAML itself before templates run and cannot be detected here.
Usage: include "slurm-operator.metricsPort" (list "operator" .Values.operator.metricsPort)
*/}}
{{- define "slurm-operator.metricsPort" -}}
{{- $name := index . 0 -}}
{{- $portStr := toString (index . 1 | default 0) -}}
{{- if not (regexMatch "^(0|[1-9][0-9]{0,4})$" $portStr) -}}
{{- fail (printf "%s.metricsPort must be an integer between 0 and 65535" $name) -}}
{{- end -}}
{{- $port := atoi $portStr -}}
{{- if gt $port 65535 -}}
{{- fail (printf "%s.metricsPort must be an integer between 0 and 65535" $name) -}}
{{- end -}}
{{- $port -}}
{{- end }}

{{/*
Common chart labels, not scoped to a component.
*/}}
{{- define "slurm-operator.labels" -}}
helm.sh/chart: {{ include "slurm-operator.chart" . }}
app.kubernetes.io/part-of: slurm-operator
app.kubernetes.io/name: {{ include "slurm-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
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
Format image reference from image object.
*/}}
{{- define "slurm.format-image" -}}
{{- $spec := index . 0 -}}
{{- $defaultTag := index . 1 -}}
{{- if kindIs "string" $spec -}}
  {{- $image := required "image is required" $spec -}}
  {{- printf $image | toString -}}
{{- else -}}
  {{- $repository := required "image repository is required" $spec.repository -}}
  {{- if $spec.digest -}}
    {{- $digest := required "image digest is required" $spec.digest -}}
    {{- printf "%s@%s" $repository $digest | toString -}}
  {{- else -}}
    {{- $tag := required "image tag is required" ($spec.tag | default $defaultTag) -}}
    {{- printf "%s:%s" $repository $tag | toString -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{/*
Define the API group
*/}}
{{- define "slurm-operator.apiGroup" -}}
{{- print "slinky.slurm.net" }}
{{- end }}
