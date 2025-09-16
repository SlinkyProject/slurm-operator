{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Define controller name
*/}}
{{- define "slurm.controller.name" -}}
{{- printf "%s-controller" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Define controller service name
*/}}
{{- define "slurm.controller.service" -}}
{{- printf "%s-controller" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Define controller port
*/}}
{{- define "slurm.controller.port" -}}
{{- print "6817" -}}
{{- end }}

{{/*
Determine controller extraConf
*/}}
{{- define "slurm.controller.extraConf" -}}
{{- $extraConf := list -}}
{{- if .Values.controller.extraConf -}}
  {{- $extraConf = splitList "\n" .Values.controller.extraConf -}}
{{- else if .Values.controller.extraConfMap -}}
  {{- $extraConf = (include "_toList" .Values.controller.extraConfMap) | splitList ";" -}}
{{- end -}}
{{- $nodesetList := list "ALL" -}}
{{- range $nodesetName, $nodeset := .Values.nodesets -}}
  {{- if $nodeset.enabled }}
    {{- $nodesetList = append $nodesetList $nodesetName }}
  {{- end }}{{- /* if $nodeset.enabled */}}
{{- end }}{{- /* range $nodeset := .Values.nodesets */}}
{{- range $partName, $part := .Values.partitions -}}
  {{- $part_nodesets := $part.nodesets | default list | uniq | sortAlpha -}}
  {{- if eq (len $part_nodesets) 0 -}}
    {{- fail (printf "partition `%s` must contain at least one NodeSet (or ALL)." $partName) }}
  {{- end -}}{{- /* if eq len $part_nodesets 0 */}}
  {{- if $part.enabled }}
    {{- range $part_nodesetName := $part_nodesets -}}
      {{- if not (has $part_nodesetName $nodesetList) }}
        {{- fail (printf "partition `%s` is referencing nodeset `%s` that does not exist or is disabled." $partName $part_nodesetName) }}
      {{- end }}{{- /* if not (has $part_nodesetName $nodesetList) */}}
    {{- end }}{{- /* range $part_nodesetName := $part_nodesets */}}
    {{- $partNodes := list -}}
    {{- range $part_nodesetName := $part_nodesets -}}
      {{- if has $part_nodesetName $nodesetList -}}
        {{- $partNodes = append $partNodes $part_nodesetName -}}
      {{- end -}}{{- /* if has $part_nodesetName $nodesetList */}}
    {{- end -}}{{- /* range $part_nodesetName := $part_nodesets */}}
    {{- $partLine := list (printf "PartitionName=%s" $partName) (printf "Nodes=%s" (join "," $partNodes)) -}}
    {{- $partConfig := list -}}
    {{- if $part.config -}}
      {{- $partConfig = list $part.config -}}
    {{- else if $part.configMap -}}
      {{- $partConfig = (include "_toList" $part.configMap) | splitList ";" -}}
    {{- end -}}
    {{- $partLine = append $partLine (join " " $partConfig) -}}
    {{- $extraConf = append $extraConf (join " " $partLine) -}}
  {{- end }}{{- /* if $part.enabled */}}
{{- end }}{{- /* range $part := .Values.partitions */}}
{{- join "\n" $extraConf -}}
{{- end }}

{{/*
Cluster config files.
*/}}
{{- define "slurm.controller.configName" -}}
{{- printf "%s-config-extra" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Worker prolog scripts.
*/}}
{{- define "slurm.controller.prologName" -}}
{{- printf "%s-prolog-scripts" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Worker epilog scripts.
*/}}
{{- define "slurm.controller.epilogName" -}}
{{- printf "%s-epilog-scripts" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Worker prolog slurmctld scripts.
*/}}
{{- define "slurm.controller.prologSlurmctldName" -}}
{{- printf "%s-prolog-slurmctld-scripts" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Worker epilog slurmctld scripts.
*/}}
{{- define "slurm.controller.epilogSlurmctldName" -}}
{{- printf "%s-epilog-slurmctld-scripts" (include "slurm.fullname" .) -}}
{{- end }}
