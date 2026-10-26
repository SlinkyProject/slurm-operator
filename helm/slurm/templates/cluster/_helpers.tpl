{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Deprecated sssdConfRef. Emitted only when sssd.secretRef or sssd.conf is set.
*/}}
{{- define "slurm.sssdConf.ref" -}}
{{- $sssd := .Values.sssd | default dict -}}
{{- $secretRef := $sssd.secretRef | default dict -}}
{{- if or $secretRef.name $sssd.conf -}}
sssdConfRef:
  name: {{ include "slurm.sssdConf.name" . }}
  key: {{ include "slurm.sssdConf.key" . }}
{{- end -}}
{{- end }}

{{/*
Define sssd.conf secret name
*/}}
{{- define "slurm.sssdConf.name" -}}
{{- $secretRef := ((.Values.sssd | default dict).secretRef) | default dict -}}
{{- if $secretRef.name -}}
{{- print $secretRef.name -}}
{{- else -}}
{{- printf "%s-sssd-conf" (include "slurm.fullname" .) -}}
{{- end -}}
{{- end }}

{{/*
Define secret key
*/}}
{{- define "slurm.sssdConf.key" -}}
{{- $secretRef := ((.Values.sssd | default dict).secretRef) | default dict -}}
{{- if $secretRef.key -}}
{{- print $secretRef.key -}}
{{- else -}}
{{- print "sssd.conf" -}}
{{- end -}}
{{- end }}
