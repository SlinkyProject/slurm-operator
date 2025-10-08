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
Common webhook labels
*/}}
{{- define "slurm-operator.webhook.labels" -}}
helm.sh/chart: {{ include "slurm-operator.chart" . }}
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
{{- if .Values.webhook.serviceAccount.create }}
{{- default (include "slurm-operator.webhook.name" .) .Values.webhook.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.webhook.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Determine operator webhook image repository
*/}}
{{- define "slurm-operator.webhook.image.repository" -}}
{{ .Values.webhook.image.repository | default "ghcr.io/slinkyproject/slurm-operator-webhook" }}
{{- end }}

{{/*
Define operator webhook image tag
*/}}
{{- define "slurm-operator.webhook.image.tag" -}}
{{ .Values.webhook.image.tag | default .Chart.Version }}
{{- end }}

{{/*
Determine operator webhook image reference (repo:tag)
*/}}
{{- define "slurm-operator.webhook.imageRef" -}}
{{ printf "%s:%s" (include "slurm-operator.webhook.image.repository" .) (include "slurm-operator.webhook.image.tag" .) | quote }}
{{- end }}

{{/*
Define operator webhook imagePullPolicy
*/}}
{{- define "slurm-operator.webhook.imagePullPolicy" -}}
{{ .Values.webhook.imagePullPolicy | default .Values.imagePullPolicy }}
{{- end }}
