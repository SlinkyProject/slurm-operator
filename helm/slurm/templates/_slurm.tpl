{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Define auth/slurm secret ref name
*/}}
{{- define "slurm.authSlurmRef.name" -}}
{{- printf "%s-auth-slurm" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Define auth/slurm secret ref key
*/}}
{{- define "slurm.authSlurmRef.key" -}}
{{- print "slurm.key" -}}
{{- end }}

{{/*
Define auth/jwt HS256 secret ref name
*/}}
{{- define "slurm.authJwtHs256Ref.name" -}}
{{- printf "%s-auth-jwths256" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Define auth/jwt HS256 secret ref key
*/}}
{{- define "slurm.authJwtHs256Ref.key" -}}
{{- print "jwt_hs256.key" -}}
{{- end }}

{{/*
Define login name
*/}}
{{- define "slurm.login.name" -}}
{{- printf "%s-login" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Define login labels
*/}}
{{- define "slurm.login.labels" -}}
app.kubernetes.io/component: login
{{ include "slurm.login.selectorLabels" . }}
{{ include "slurm.labels" . }}
{{- end }}

{{/*
Define login selectorLabels
*/}}
{{- define "slurm.login.selectorLabels" -}}
app.kubernetes.io/name: login
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
