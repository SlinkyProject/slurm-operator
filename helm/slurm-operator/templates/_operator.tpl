{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

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
Determine operator image reference (repo:tag)
*/}}
{{- define "slurm-operator.operator.imageRef" -}}
{{ printf "%s:%s" (include "slurm-operator.operator.image.repository" .) (include "slurm-operator.operator.image.tag" .) | quote }}
{{- end }}

{{/*
Define operator imagePullPolicy
*/}}
{{- define "slurm-operator.operator.imagePullPolicy" -}}
{{ .Values.operator.imagePullPolicy | default .Values.imagePullPolicy }}
{{- end }}
