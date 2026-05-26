{{- /*
SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "slurm-operator.webhook.name" -}}
{{ printf "%s-webhook" (include "slurm-operator.name" .) }}
{{- end }}

{{/*
Common webhook labels
*/}}
{{- define "slurm-operator.webhook.labels" -}}
helm.sh/chart: {{ include "slurm-operator.chart" . }}
app.kubernetes.io/part-of: slurm-operator
{{ include "slurm-operator.webhook.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector webhook labels
*/}}
{{- define "slurm-operator.webhook.selectorLabels" -}}
app.kubernetes.io/name: {{ include "slurm-operator.webhook.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the webhook service account to use
*/}}
{{- define "slurm-operator.webhook.serviceAccountName" -}}
{{- $serviceAccount := .Values.webhook.serviceAccount | default dict -}}
{{- if $serviceAccount.create }}
{{- default (include "slurm-operator.webhook.name" .) $serviceAccount.name }}
{{- else }}
{{- default "default" $serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Define operator webhook imagePullPolicy
*/}}
{{- define "slurm-operator.webhook.imagePullPolicy" -}}
{{ .Values.webhook.imagePullPolicy | default .Values.imagePullPolicy }}
{{- end }}

{{/*
Render a webhook namespaceSelector.
Args (dict):
  root          - root context
  override      - a namespaceSelector to render verbatim (optional)
  extraExcludes - extra namespaces to add to the NotIn exclusions (optional)
When override is set it is rendered verbatim. Otherwise the selector is built from
webhook.namespaces (In) and the kube-system/kube-node-lease exclusions (NotIn), plus
any extraExcludes.
*/}}
{{- define "slurm-operator.webhook.namespaceSelector" -}}
{{- $root := .root -}}
{{- $override := .override -}}
{{- $extraExcludes := .extraExcludes | default list -}}
{{- if $override -}}
{{- toYaml $override -}}
{{- else -}}
matchExpressions:
{{- $namespaceList := nospace $root.Values.webhook.namespaces | splitList "," -}}
{{- if $root.Values.webhook.namespaces }}
  - key: kubernetes.io/metadata.name
    operator: In
    values:
      {{- $namespaceList | toYaml | nindent 6 }}
{{- end }}
  - key: kubernetes.io/metadata.name
    operator: NotIn
    values:
      {{- concat (list "kube-system" "kube-node-lease") $extraExcludes | toYaml | nindent 6 }}
{{- end -}}
{{- end -}}

{{/*
Validate cert-provisioning modes. Called from every template that
participates in provisioning so the error surfaces no matter which
template Helm renders first.
*/}}
{{- define "slurm-operator.webhook.validateModes" -}}
{{- if and .Values.certManager.enabled .Values.externalCertInjection.enabled -}}
{{- fail "certManager.enabled and externalCertInjection.enabled are mutually exclusive: both would manage the webhook TLS Secret. Pick one." -}}
{{- end -}}
{{- if and .Values.externalCertInjection.enabled (not .Values.externalCertInjection.secretName) -}}
{{- fail "externalCertInjection.enabled=true but externalCertInjection.secretName is empty." -}}
{{- end -}}
{{- end }}

{{/*
Name of the Secret that holds the webhook serving cert.
*/}}
{{- define "slurm-operator.webhook.tlsSecretName" -}}
{{- if .Values.externalCertInjection.enabled -}}
{{ .Values.externalCertInjection.secretName }}
{{- else -}}
{{ .Values.certManager.secretName }}
{{- end -}}
{{- end }}

{{/*
Chart-managed cert-manager annotations. Empty when certManager is off.
*/}}
{{- define "slurm-operator.webhook.certManagerAnnotations" -}}
{{- $ann := dict -}}
{{- if .Values.certManager.enabled -}}
{{- $ref := printf "%s/%s" (include "slurm-operator.namespace" .) .Values.certManager.secretName -}}
{{- $_ := set $ann "certmanager.k8s.io/inject-ca-from" $ref -}}
{{- $_ := set $ann "cert-manager.io/inject-ca-from" $ref -}}
{{- end -}}
{{- toYaml $ann -}}
{{- end }}

{{/*
ValidatingWebhookConfiguration annotations. User keys win on collision.
*/}}
{{- define "slurm-operator.webhook.validatingAnnotations" -}}
{{- $userAnn := .Values.webhook.validatingAnnotations | default dict -}}
{{- $cmAnn := include "slurm-operator.webhook.certManagerAnnotations" . | fromYaml -}}
{{- $ann := merge dict $userAnn $cmAnn -}}
{{- if $ann -}}
{{- toYaml $ann -}}
{{- end -}}
{{- end }}

{{/*
MutatingWebhookConfiguration annotations. User keys win on collision.
*/}}
{{- define "slurm-operator.webhook.mutatingAnnotations" -}}
{{- $userAnn := .Values.webhook.mutatingAnnotations | default dict -}}
{{- $cmAnn := include "slurm-operator.webhook.certManagerAnnotations" . | fromYaml -}}
{{- $ann := merge dict $userAnn $cmAnn -}}
{{- if $ann -}}
{{- toYaml $ann -}}
{{- end -}}
{{- end }}

{{/*
Base64-encoded CA bundle read from the external TLS Secret.
NOTE: `lookup` returns nil during `helm template` and `--dry-run=client`,
so this helper will fail with "not found" in those contexts even when
the Secret exists. Use `--dry-run=server` or the annotation pass-through.
*/}}
{{- define "slurm-operator.webhook.externalCABundle" -}}
{{- $ns := include "slurm-operator.namespace" . -}}
{{- $name := .Values.externalCertInjection.secretName -}}
{{- $secret := lookup "v1" "Secret" $ns $name -}}
{{- if not $secret -}}
{{- fail (printf "externalCertInjection Secret %q in namespace %q was not found (use --dry-run=server; see docs/installation.md)." $name $ns) -}}
{{- end -}}
{{- if ne $secret.type "kubernetes.io/tls" -}}
{{- fail (printf "externalCertInjection Secret %q in namespace %q must be of type kubernetes.io/tls, got %q." $name $ns $secret.type) -}}
{{- end -}}
{{- range $key := list "tls.crt" "tls.key" "ca.crt" -}}
{{- if not (index $secret.data $key) -}}
{{- fail (printf "externalCertInjection Secret %q in namespace %q is missing the %q key." $name $ns $key) -}}
{{- end -}}
{{- end -}}
{{- index $secret.data "ca.crt" -}}
{{- end }}
