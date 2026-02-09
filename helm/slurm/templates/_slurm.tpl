{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Define auth/slurm secret ref name
*/}}
{{- define "slurm.authSlurmRef.name" -}}
{{- if .Values.slurmKey.secretRef.name }}
{{- .Values.slurmKey.secretRef.name }}
{{- else }}
{{- printf "%s-auth-slurm" (include "slurm.fullname" .) -}}
{{- end }}
{{- end }}

{{/*
Define auth/slurm secret ref key
*/}}
{{- define "slurm.authSlurmRef.key" -}}
{{- if .Values.slurmKey.secretRef.key }}
{{- .Values.slurmKey.secretRef.key }}
{{- else }}
{{- print "slurm.key" -}}
{{- end }}
{{- end }}

{{/*
Define auth/jwt HS256 secret ref name
*/}}
{{- define "slurm.authJwtHs256Ref.name" -}}
{{- if .Values.jwtHs256Key.secretRef.name }}
{{- .Values.jwtHs256Key.secretRef.name }}
{{- else }}
{{- printf "%s-auth-jwths256" (include "slurm.fullname" .) -}}
{{- end }}
{{- end }}

{{/*
Define auth/jwt HS256 secret ref key
*/}}
{{- define "slurm.authJwtHs256Ref.key" -}}
{{- if .Values.jwtHs256Key.secretRef.key }}
{{- .Values.jwtHs256Key.secretRef.key }}
{{- else }}
{{- print "jwt_hs256.key" -}}
{{- end }}
{{- end }}
