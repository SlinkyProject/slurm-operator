{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Define restapi name
*/}}
{{- define "slurm.restapi.name" -}}
{{- printf "%s-restapi" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Define restapi port
*/}}
{{- define "slurm.restapi.port" -}}
{{- print "6820" -}}
{{- end }}

{{/*
RestApi selector labels
*/}}
{{- define "slurm.restapi.selectorLabels" -}}
app.kubernetes.io/name: slurmrestd
app.kubernetes.io/instance: {{ include "slurm.fullname" . }}
{{- end }}
