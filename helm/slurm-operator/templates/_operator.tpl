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
{{- if .Values.operator.serviceAccount.create }}
{{- default (include "slurm-operator.fullname" .) .Values.operator.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.operator.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Determine operator image repository
*/}}
{{- define "slurm-operator.operator.image.repository" -}}
{{ .Values.operator.image.repository | default "ghcr.io/slinkyproject/slurm-operator" }}
{{- end }}

{{/*
Define operator image tag
*/}}
{{- define "slurm-operator.operator.image.tag" -}}
{{ .Values.operator.image.tag | default .Chart.Version }}
{{- end }}

{{/*
Define operator image digest
*/}}
{{- define "slurm-operator.operator.image.digest" -}}
{{ .Values.operator.image.digest | default "" }}
{{- end }}

{{/*
Determine operator image reference. Prefers `repo@digest` when digest is set,
falling back to `repo:tag` otherwise.
*/}}
{{- define "slurm-operator.operator.imageRef" -}}
{{- $repo := include "slurm-operator.operator.image.repository" . | trim -}}
{{- $digest := include "slurm-operator.operator.image.digest" . | trim -}}
{{- if $digest -}}
{{ printf "%s@%s" $repo $digest | quote }}
{{- else -}}
{{- $tag := include "slurm-operator.operator.image.tag" . | trim -}}
{{ printf "%s:%s" $repo $tag | quote }}
{{- end -}}
{{- end }}

{{/*
Define operator imagePullPolicy
*/}}
{{- define "slurm-operator.operator.imagePullPolicy" -}}
{{ .Values.operator.imagePullPolicy | default .Values.imagePullPolicy }}
{{- end }}
