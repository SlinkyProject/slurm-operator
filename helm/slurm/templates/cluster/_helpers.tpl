{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Define sssd.conf secret name
*/}}
{{- define "slurm.sssdConf.name" -}}
{{- $secretRef := .Values.sssd.secretRef | default dict -}}
{{- if $secretRef.name -}}
  {{- print $secretRef.name -}}
{{- else -}}
  {{- printf "%s-sssd-conf" (include "slurm.fullname" .) -}}
{{- end }}
{{- end }}

{{/*
Define secret key
*/}}
{{- define "slurm.sssdConf.key" -}}
{{- $secretRef := .Values.sssd.secretRef | default dict -}}
{{- if $secretRef.key -}}
  {{- print $secretRef.key -}}
{{- else -}}
  {{- print "sssd.conf" -}}
{{- end }}
{{- end }}
