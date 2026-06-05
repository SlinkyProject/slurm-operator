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
RestApi selector labels.
These must match the labels the operator applies to the slurmrestd pods,
otherwise the PodDisruptionBudget would select no pods.
*/}}
{{- define "slurm.restapi.selectorLabels" -}}
app.kubernetes.io/name: slurmrestd
app.kubernetes.io/instance: {{ include "slurm.fullname" . }}
{{- end }}
