{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Common operator labels
*/}}
{{- define "slurm-operator.operator.labels" -}}
helm.sh/chart: {{ include "slurm-operator.chart" . }}
app.kubernetes.io/part-of: slurm-operator
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
{{- $serviceAccount := .Values.operator.serviceAccount | default dict -}}
{{- if $serviceAccount.create }}
{{- default (include "slurm-operator.fullname" .) $serviceAccount.name }}
{{- else }}
{{- default "default" $serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Define operator imagePullPolicy
*/}}
{{- define "slurm-operator.operator.imagePullPolicy" -}}
{{ .Values.operator.imagePullPolicy | default .Values.imagePullPolicy }}
{{- end }}
