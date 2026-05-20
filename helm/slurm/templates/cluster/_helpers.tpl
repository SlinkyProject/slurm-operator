{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Define the API group
*/}}
{{- define "slurm.apiGroup" -}}
{{- print "slinky.slurm.net" }}
{{- end }}

{{/*
Define sssd.conf secret name
*/}}
{{- define "slurm.sssdConf.name" -}}
{{- if .Values.sssd.secretRef -}}
  {{- print (get .Values.sssd.secretRef "name") -}}
{{- else -}}
  {{- printf "%s-sssd-conf" (include "slurm.fullname" .) -}}
{{- end }}
{{- end }}

{{/*
Define secret key
*/}}
{{- define "slurm.sssdConf.key" -}}
{{- if .Values.sssd.secretRef -}}
  {{- print (get .Values.sssd.secretRef "key") -}}
{{- else -}}
  {{- print "sssd.conf" -}}
{{- end }}
{{- end }}
