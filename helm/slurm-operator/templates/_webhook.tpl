{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "slurm-operator.webhook.name" -}}
{{ printf "%s-webhook" (include "slurm-operator.name" .) }}
{{- end }}

{{/*
Release-scoped webhook name, for cluster-scoped objects that would otherwise
collide between two releases of this chart in different namespaces.
The base is truncated to 55 chars (63 minus "-webhook") so that the suffix
survives a long release name and still distinguishes the webhook's objects.
*/}}
{{- define "slurm-operator.webhook.fullname" -}}
{{- $base := include "slurm-operator.fullname" . | trunc 55 | trimSuffix "-" -}}
{{- printf "%s-webhook" $base -}}
{{- end }}

{{/*
Common webhook labels
*/}}
{{- define "slurm-operator.webhook.labels" -}}
helm.sh/chart: {{ include "slurm-operator.chart" . }}
app.kubernetes.io/part-of: slurm-operator
{{ include "slurm-operator.webhook.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector webhook labels
*/}}
{{- define "slurm-operator.webhook.selectorLabels" -}}
app.kubernetes.io/name: {{ include "slurm-operator.webhook.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the webhook service account to use
*/}}
{{- define "slurm-operator.webhook.serviceAccountName" -}}
{{- $serviceAccount := .Values.webhook.serviceAccount | default dict -}}
{{- if $serviceAccount.create }}
{{- default (include "slurm-operator.webhook.name" .) $serviceAccount.name }}
{{- else }}
{{- default "default" $serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Define operator webhook imagePullPolicy
*/}}
{{- define "slurm-operator.webhook.imagePullPolicy" -}}
{{ .Values.webhook.imagePullPolicy | default .Values.imagePullPolicy }}
{{- end }}
