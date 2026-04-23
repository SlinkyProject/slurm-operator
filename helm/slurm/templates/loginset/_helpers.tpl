{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Define login name
*/}}
{{- define "slurm.login.name" -}}
{{- printf "%s-login" (include "slurm.fullname" .) -}}
{{- end }}

{{- /*
Return a LoginSet value map with chart-wide defaults applied.

Input: a dict with two keys:
  - ctx:   the root chart context (typically $ from the caller)
  - entry: the per-key entry from .Values.loginsets

Deep-merges `entry` onto `.Values.loginsets._defaults` so every
LoginSet entry inherits the same defaults regardless of its key
name. mustMergeOverwrite surfaces type mismatches as template
render errors instead of returning a partial/nil result.

Output is YAML-serialized because `include` can only return a
string. Callers must pipe through `| fromYaml` to hydrate the map,
e.g.:

  $loginset = include "slurm.loginset.withDefaults" (dict "ctx" $ "entry" $loginset) | fromYaml
*/}}
{{- define "slurm.loginset.withDefaults" -}}
{{- $defaults := index .ctx.Values.loginsets "_defaults" | default dict -}}
{{- $entry := .entry | default dict -}}
{{- mustMergeOverwrite (deepCopy $defaults) $entry | toYaml -}}
{{- end -}}
