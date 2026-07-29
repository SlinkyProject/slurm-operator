{{/*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Cluster config files.
*/}}
{{- define "slurm.vendor.google.a3ultra.enabled" -}}
{{- if gt (len (.Values.vendor.google | dig "a3ultra" list)) 0 -}}
  {{- print "true" -}}
{{- end -}}
{{- end }}

{{/*
Cluster config files.
*/}}
{{- define "slurm.vendor.google.a3ultra.configName" -}}
{{- printf "%s-config-a3ultra" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Return a nodeset values patch for A3 Ultra, or empty when not applicable.
*/}}
{{- define "slurm.vendor.google.a3ultra.nodesetPatch" -}}
{{- $root := .root -}}
{{- $key := .key -}}
{{- $a3ultraConfig := dict -}}
{{- range $config := ($root.Values.vendor.google | dig "a3ultra" list) -}}
  {{- if eq (get $config "targetNodeSet" | default "") $key -}}
    {{- $a3ultraConfig = $config -}}
  {{- end -}}
{{- end -}}
{{- if $a3ultraConfig -}}
  {{- $networks := get $a3ultraConfig "networks" | default list -}}
  {{- if ne (len $networks) 9 -}}
    {{- fail (printf "Google Cloud GKE A3 Ultra requires exactly 9 networks, but %d were specified." (len $networks)) -}}
  {{- end -}}
  {{- $_ := set $root "a3ultraConfig" $a3ultraConfig -}}
  {{- tpl ($root.Files.Get "_vendor/google/a3ultra/snippets/nodeset.yaml") $root -}}
{{- end -}}
{{- end }}
