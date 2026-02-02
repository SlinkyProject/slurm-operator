{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Define auth/slurm secret ref name
*/}}
{{- define "slurm.authSlurmRef.name" -}}
{{- .Values.slurmKeyRef.name -}}
{{- end }}

{{/*
Define auth/slurm secret ref key
*/}}
{{- define "slurm.authSlurmRef.key" -}}
{{- .Values.slurmKeyRef.key -}}
{{- end }}

{{/*
Define auth/jwt HS256 secret ref name
*/}}
{{- define "slurm.authJwtHs256Ref.name" -}}
{{- .Values.jwtHs256KeyRef.name -}}
{{- end }}

{{/*
Define auth/jwt HS256 secret ref key
*/}}
{{- define "slurm.authJwtHs256Ref.key" -}}
{{- .Values.jwtHs256KeyRef.key -}}
{{- end }}
