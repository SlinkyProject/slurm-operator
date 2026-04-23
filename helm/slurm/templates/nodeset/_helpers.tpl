{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Define worker name
*/}}
{{- define "slurm.worker.name" -}}
{{- printf "%s-worker" (include "slurm.fullname" .) -}}
{{- end }}

{{/*
Define worker port
*/}}
{{- define "slurm.worker.port" -}}
{{- print "6818" -}}
{{- end }}

{{/*
Determine worker extraConf (e.g. `--conf <extraConf>`)
*/}}
{{- define "slurm.worker.extraConf" -}}
{{- $extraConf := list -}}
{{- if .extraConf -}}
  {{- $extraConf = splitList " " .extraConf -}}
{{- else if .extraConfMap -}}
  {{- $extraConf = (include "_toList" .extraConfMap) | splitList ";" -}}
{{- end }}
{{- join " " $extraConf -}}
{{- end }}

{{/*
Determine worker partition config
*/}}
{{- define "slurm.worker.partitionConfig" -}}
{{- $config := list -}}
{{- if .config -}}
  {{- $config = list .config -}}
{{- else if .configMap -}}
  {{- $config = (include "_toList" .configMap) | splitList ";" -}}
{{- end }}
{{- join " " $config -}}
{{- end }}

{{- /*
Return a NodeSet value map with chart-wide defaults applied.

Input: a dict with two keys:
  - ctx:   the root chart context (typically $ from the caller)
  - entry: the per-key entry from .Values.nodesets

Deep-merges `entry` onto `.Values.nodesets._defaults` so every
NodeSet entry inherits the same defaults regardless of its key
name. mustMergeOverwrite surfaces type mismatches as template
render errors instead of returning a partial/nil result.

Output is YAML-serialized because `include` can only return a
string. Callers must pipe through `| fromYaml` to hydrate the map,
e.g.:

  $nodeset = include "slurm.nodeset.withDefaults" (dict "ctx" $ "entry" $nodeset) | fromYaml
*/}}
{{- define "slurm.nodeset.withDefaults" -}}
{{- $defaults := index .ctx.Values.nodesets "_defaults" | default dict -}}
{{- $entry := .entry | default dict -}}
{{- mustMergeOverwrite (deepCopy $defaults) $entry | toYaml -}}
{{- end -}}
