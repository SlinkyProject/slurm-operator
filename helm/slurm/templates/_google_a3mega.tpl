{{/*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{- define "slurm.vendor.google.a3mega" -}}
{{- $root := .root -}}
{{- $key := .key -}}
{{- $nodeset := .nodeset -}}
{{- $metadata := .metadata -}}
{{- $podSpec := .podSpec -}}

{{- $a3megaConfig := dict -}}
{{- range $config := ($root.Values.vendor.google | dig "a3mega" list) -}}
  {{- if eq (get $config "targetNodeSet" | default "") $key -}}
    {{- $a3megaConfig = $config -}}
  {{- end -}}
{{- end -}}
{{- if $a3megaConfig }}
  {{- $networks := get $a3megaConfig "networks" | default list -}}
  {{- if ne (len $networks) 9 -}}
    {{- fail (printf "Google Cloud GKE A3 Mega requires exactly 9 networks, but %d were specified." (len $networks)) -}}
  {{- end -}}

  {{- /* Inject Annotations */ -}}
  {{- $_ := set $root "a3megaConfig" $a3megaConfig -}}
  {{- $a3Annotations := tpl ($root.Files.Get "_vendor/google/a3mega/annotations.yaml") $root | fromYaml -}}
  {{- $_ := set $metadata "annotations" (merge (get $metadata "annotations" | default dict) $a3Annotations) -}}

  {{- /* Inject NodeSelector for node pool */ -}}
  {{- if get $a3megaConfig "nodePool" }}
    {{- $nodeSelector := $podSpec.nodeSelector | default dict -}}
    {{- if not (hasKey $nodeSelector "cloud.google.com/gke-nodepool") -}}
      {{- $_ := set $nodeSelector "cloud.google.com/gke-nodepool" (get $a3megaConfig "nodePool") -}}
      {{- $_ := set $podSpec "nodeSelector" $nodeSelector -}}
    {{- end -}}
  {{- end }}

  {{- /* Inject slurmd resources limits/requests automatically */ -}}
  {{- $resources := $nodeset.slurmd.resources | default dict -}}
  {{- $limits := $resources.limits | default dict -}}
  {{- $requests := $resources.requests | default dict -}}
  {{- if not (hasKey $limits "nvidia.com/gpu") -}}
    {{- $_ := set $limits "nvidia.com/gpu" 8 -}}
  {{- end -}}
  {{- if not (hasKey $requests "nvidia.com/gpu") -}}
    {{- $_ := set $requests "nvidia.com/gpu" 8 -}}
  {{- end -}}
  {{- $_ := set $resources "limits" $limits -}}
  {{- $_ := set $resources "requests" $requests -}}
  {{- $_ := set $nodeset.slurmd "resources" $resources -}}

  {{- /* Inject extraConfMap with Gres */ -}}
  {{- $extraConfMap := $nodeset.extraConfMap | default dict -}}
  {{- if not (hasKey $extraConfMap "Gres") -}}
    {{- $_ := set $extraConfMap "Gres" "gpu:nvidia_h100_80gb_hbm3:8" -}}
    {{- $_ := set $nodeset "extraConfMap" $extraConfMap -}}
  {{- end -}}

  {{- /* Inject tcpxo-daemon sidecar container */ -}}
  {{- $containers := $podSpec.containers | default list -}}
  {{- $a3Containers := tpl ($root.Files.Get "_vendor/google/a3mega/containers.yaml") $root | fromYamlArray -}}
  {{- $filteredA3Containers := list -}}
  {{- range $a3c := $a3Containers -}}
    {{- $exists := false -}}
    {{- range $c := $containers -}}{{ if eq $c.name $a3c.name }}{{ $exists = true }}{{ end }}{{ end -}}
    {{- if not $exists -}}{{ $filteredA3Containers = append $filteredA3Containers $a3c }}{{ end -}}
  {{- end -}}
  {{- $_ := set $podSpec "containers" (concat $containers $filteredA3Containers) -}}

  {{- /* Inject Host and EmptyDir Volumes */ -}}
  {{- $volumes := $podSpec.volumes | default list -}}
  {{- $a3Volumes := tpl ($root.Files.Get "_vendor/google/a3mega/volumes.yaml") $root | fromYamlArray -}}
  {{- $filteredA3Volumes := list -}}
  {{- range $a3v := $a3Volumes -}}
    {{- $exists := false -}}
    {{- range $v := $volumes -}}{{ if eq $v.name $a3v.name }}{{ $exists = true }}{{ end }}{{ end -}}
    {{- if not $exists -}}{{ $filteredA3Volumes = append $filteredA3Volumes $a3v }}{{ end -}}
  {{- end -}}
  {{- $_ := set $podSpec "volumes" (concat $volumes $filteredA3Volumes) -}}

  {{- /* Inject slurmd volume mounts */ -}}
  {{- $slurmdVolumeMounts := $nodeset.slurmd.volumeMounts | default list -}}
  {{- $a3SlurmdVolumeMounts := tpl ($root.Files.Get "_vendor/google/a3mega/slurmdVolumeMounts.yaml") $root | fromYamlArray -}}
  {{- $filteredA3SlurmdVolumeMounts := list -}}
  {{- range $a3vm := $a3SlurmdVolumeMounts -}}
    {{- $exists := false -}}
    {{- range $vm := $slurmdVolumeMounts -}}{{ if eq $vm.name $a3vm.name }}{{ $exists = true }}{{ end }}{{ end -}}
    {{- if not $exists -}}{{ $filteredA3SlurmdVolumeMounts = append $filteredA3SlurmdVolumeMounts $a3vm }}{{ end -}}
  {{- end -}}
  {{- $_ := set $nodeset.slurmd "volumeMounts" (concat $slurmdVolumeMounts $filteredA3SlurmdVolumeMounts) -}}

  {{- /* Inject slurmd environment variables */ -}}
  {{- $slurmdEnv := $nodeset.slurmd.env | default list -}}
  {{- $a3SlurmdEnv := tpl ($root.Files.Get "_vendor/google/a3mega/slurmdEnv.yaml") $root | fromYamlArray -}}
  {{- $filteredA3SlurmdEnv := list -}}
  {{- range $a3e := $a3SlurmdEnv -}}
    {{- $exists := false -}}
    {{- range $e := $slurmdEnv -}}{{ if eq $e.name $a3e.name }}{{ $exists = true }}{{ end }}{{ end -}}
    {{- if not $exists -}}{{ $filteredA3SlurmdEnv = append $filteredA3SlurmdEnv $a3e }}{{ end -}}
  {{- end -}}
  {{- $_ := set $nodeset.slurmd "env" (concat $slurmdEnv $filteredA3SlurmdEnv) -}}

{{- end }}
{{- end -}}
