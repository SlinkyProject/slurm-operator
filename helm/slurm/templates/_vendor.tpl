{{/*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
slurm.deepMerge

Deep-merge two Helm values structures.

Rules:
- map + map     => recurse per key
- slice + slice => merge list items by "key" or "name", append the rest
- otherwise     => right wins

Returns a JSON envelope {"_v":...} so callers can always parse with fromJson
(maps and slices both work; avoids fromJson vs fromJsonArray branching).
*/}}
{{- define "slurm.deepMerge" -}}
{{- $left := index . 0 -}}
{{- $right := index . 1 -}}

{{- if kindIs "invalid" $left -}}
  {{- dict "_v" $right | mustToJson -}}
{{- else if kindIs "invalid" $right -}}
  {{- dict "_v" $left | mustToJson -}}
{{- else if and (kindIs "map" $left) (kindIs "map" $right) -}}
  {{- $out := deepCopy $left -}}
  {{- range $k, $rv := $right -}}
    {{- if hasKey $out $k -}}
      {{- $lv := get $out $k -}}
      {{- $merged := include "slurm.deepMerge" (list $lv $rv) | fromJson -}}
      {{- $_ := set $out $k $merged._v -}}
    {{- else -}}
      {{- $_ := set $out $k $rv -}}
    {{- end -}}
  {{- end -}}
  {{- dict "_v" $out | mustToJson -}}

{{- else if and (kindIs "slice" $left) (kindIs "slice" $right) -}}
  {{- include "slurm.deepMergeList" (list $left $right) -}}

{{- else -}}
  {{- dict "_v" $right | mustToJson -}}
{{- end -}}
{{- end -}}

{{/*
slurm.itemMergeId

Returns a stable merge id for a list item:
- prefer "key"
- else use "name"
- else empty string (item is appended, not merged)
*/}}
{{- define "slurm.itemMergeId" -}}
{{- $item := . -}}
{{- if kindIs "map" $item -}}
  {{- if hasKey $item "key" -}}
    {{- printf "%v" (get $item "key") -}}
  {{- else if hasKey $item "name" -}}
    {{- printf "%v" (get $item "name") -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{/*
slurm.deepMergeList

Merge two lists. Items with the same merge id are deep-merged; others append.
Order: keyed items from left (updated by right), new keyed items from right,
then non-keyed left items, then non-keyed right items.
*/}}
{{- define "slurm.deepMergeList" -}}
{{- $left := index . 0 -}}
{{- $right := index . 1 -}}
{{- if kindIs "invalid" $left -}}{{- $left = list -}}{{- end -}}
{{- if kindIs "invalid" $right -}}{{- $right = list -}}{{- end -}}

{{- $mergedById := dict -}}
{{- $orderedIds := list -}}
{{- $leftExtras := list -}}
{{- $rightExtras := list -}}

{{- range $item := $left -}}
  {{- $id := include "slurm.itemMergeId" $item -}}
  {{- if and (ne $id "") (kindIs "map" $item) -}}
    {{- $_ := set $mergedById $id $item -}}
    {{- $orderedIds = append $orderedIds $id -}}
  {{- else -}}
    {{- $leftExtras = append $leftExtras $item -}}
  {{- end -}}
{{- end -}}

{{- range $item := $right -}}
  {{- $id := include "slurm.itemMergeId" $item -}}
  {{- if and (ne $id "") (kindIs "map" $item) -}}
    {{- if hasKey $mergedById $id -}}
      {{- $existing := get $mergedById $id -}}
      {{- $combined := include "slurm.deepMerge" (list $existing $item) | fromJson -}}
      {{- $_ := set $mergedById $id $combined._v -}}
    {{- else -}}
      {{- $_ := set $mergedById $id $item -}}
      {{- $orderedIds = append $orderedIds $id -}}
    {{- end -}}
  {{- else -}}
    {{- $rightExtras = append $rightExtras $item -}}
  {{- end -}}
{{- end -}}

{{- $out := list -}}
{{- range $id := $orderedIds -}}
  {{- $out = append $out (get $mergedById $id) -}}
{{- end -}}
{{- range $item := $leftExtras -}}
  {{- $out = append $out $item -}}
{{- end -}}
{{- range $item := $rightExtras -}}
  {{- $out = append $out $item -}}
{{- end -}}

{{- dict "_v" $out | mustToJson -}}
{{- end -}}

{{/*
slurm.nodeset.applyPatch

Merge a YAML patch document into a nodeset values object.
Returns the merged nodeset as YAML, or the original nodeset when patch is empty.
*/}}
{{- define "slurm.nodeset.applyPatch" -}}
{{- $nodeset := .nodeset -}}
{{- $patchYaml := .patchYaml | trim -}}
{{- if $patchYaml -}}
  {{- $patch := $patchYaml | fromYaml -}}
  {{- $merged := include "slurm.deepMerge" (list $nodeset $patch) | fromJson -}}
  {{- $merged._v | toYaml -}}
{{- else -}}
  {{- $nodeset | toYaml -}}
{{- end -}}
{{- end -}}

{{/*
slurm.nodeset.applyVendorPatches

Apply vendor nodeset patches in a fixed order.

Vendor contract:
- Implement "vendor.<vendor>.nodesetPatch" accepting (dict "root" $ "key" $key "nodeset" $nodeset)
- Return a YAML fragment in nodeset values shape (podSpec, slurmd, metadata, ...)
- Return empty output when the patch does not apply
- Keep validation and tpl context setup inside the vendor helper
- Register new vendors below in merge order
*/}}
{{- define "slurm.nodeset.applyVendorPatches" -}}
{{- $root := .root -}}
{{- $key := .key -}}
{{- $nodeset := .nodeset -}}
{{- $ctx := dict "root" $root "key" $key "nodeset" $nodeset -}}

{{/* NVIDIA DCGM */}}
{{- $nodeset = include "slurm.nodeset.applyPatch" (dict "nodeset" $nodeset "patchYaml" (include "slurm.vendor.nvidia.dcgm.nodesetPatch" $ctx)) | fromYaml -}}

{{/* Google A3 Mega */}}
{{- $nodeset = include "slurm.nodeset.applyPatch" (dict "nodeset" $nodeset "patchYaml" (include "slurm.vendor.google.a3mega.nodesetPatch" $ctx)) | fromYaml -}}

{{/* Google A3 Ultra */}}
{{- $nodeset = include "slurm.nodeset.applyPatch" (dict "nodeset" $nodeset "patchYaml" (include "slurm.vendor.google.a3ultra.nodesetPatch" $ctx)) | fromYaml -}}

{{- $nodeset | toYaml -}}
{{- end -}}
