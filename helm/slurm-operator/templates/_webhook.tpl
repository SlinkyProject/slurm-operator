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

{{/*
Render a webhook namespaceSelector.
Args (dict):
  root          - root context
  override      - a namespaceSelector to render verbatim (optional)
  extraExcludes - extra namespaces to add to the NotIn exclusions (optional)
When override is set it is rendered verbatim. Otherwise the selector is built from
webhook.namespaces (In) and the kube-system/kube-node-lease exclusions (NotIn), plus
any extraExcludes.
*/}}
{{- define "slurm-operator.webhook.namespaceSelector" -}}
{{- $root := .root -}}
{{- $override := .override -}}
{{- $extraExcludes := .extraExcludes | default list -}}
{{- if $override -}}
{{- toYaml $override -}}
{{- else -}}
matchExpressions:
{{- $namespaceList := nospace $root.Values.webhook.namespaces | splitList "," -}}
{{- if $root.Values.webhook.namespaces }}
  - key: kubernetes.io/metadata.name
    operator: In
    values:
      {{- $namespaceList | toYaml | nindent 6 }}
{{- end }}
  - key: kubernetes.io/metadata.name
    operator: NotIn
    values:
      {{- concat (list "kube-system" "kube-node-lease") $extraExcludes | toYaml | nindent 6 }}
{{- end -}}
{{- end -}}
