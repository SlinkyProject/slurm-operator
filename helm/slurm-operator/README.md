# slurm-operator

![Version: 1.2.0-rc1](https://img.shields.io/badge/Version-1.2.0--rc1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 26.05](https://img.shields.io/badge/AppVersion-26.05-informational?style=flat-square)

Slurm Operator

**Homepage:** <https://slinky.schedmd.com/>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| SchedMD LLC. | <slinky@schedmd.com> | <https://support.schedmd.com/> |

## Source Code

* <https://github.com/SlinkyProject/slurm-operator>

## Requirements

Kubernetes: `>= 1.29.0-0`

| Repository | Name | Version |
|------------|------|---------|
| file://../slurm-operator-crds | slurm-operator-crds | 1.2.0-rc1 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| certManager.duration | string | `"43800h0m0s"` | Duration of certificate life. |
| certManager.enabled | bool | `true` | Enable cert-manager for certificate management. |
| certManager.renewBefore | string | `"8760h0m0s"` | Certificate renewal time. Should be before the expiration. |
| certManager.secretName | string | `"slurm-operator-webhook-ca"` | The secret to be (created and) mounted. |
| crds | object | `{"enabled":false}` | Configure Custom Resource Definitions (CRDs). |
| crds.enabled | bool | `false` | Whether this helm chart should manage the CRD and its upgrades. |
| extraObjects | list | `[]` | Extra Kubernetes objects to deploy alongside the chart. Each entry is rendered as a standalone Kubernetes object. Supports Helm templating (e.g. {{ .Release.Namespace }}). |
| fullnameOverride | string | `""` | Overrides the full name of the release. |
| imagePullPolicy | string | `"IfNotPresent"` | Set the default image pull policy. |
| imagePullSecrets | list | `[]` | Sets the image pull secrets. Ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/ |
| nameOverride | string | `""` | Overrides the name of the release. |
| namespaceOverride | string | `""` | Overrides the namespace of the release. |
| operator.accountingWorkers | int | `4` | Set the max concurrent workers for the Accounting controller. |
| operator.affinity | object | `{}` | Affinity for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| operator.controllerWorkers | int | `4` | Set the max concurrent workers for the Controller controller. |
| operator.enabled | bool | `true` | Enables the operator. |
| operator.healthPort | int | `8081` | Set the port used for health checks. |
| operator.image | object | `{"digest":null,"repository":"ghcr.io/slinkyproject/slurm-operator","tag":null}` | The image to use. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| operator.imagePullPolicy | string | `"IfNotPresent"` | Set the image pull policy. |
| operator.leaderElection | bool | `true` | Enable leader election for slurm-operator |
| operator.logLevel | string | `"info"` | Set the log level by string (e.g. error, info, debug) or number (e.g. 1..5). |
| operator.loginsetWorkers | int | `4` | Set the max concurrent workers for the LoginSet controller. |
| operator.metricsPort | int | `8080` | Set the port used by the metrics server. Value of "0" will disable it. |
| operator.metricsSecure | bool | `false` | Serve the metrics endpoint securely via HTTPS with authn/authz. Requires metricsPort to be non-zero. Scraping clients must present a token authorized to access /metrics (e.g. bound to the metrics-reader ClusterRole). The endpoint uses a generated self-signed certificate, so scrapers must skip TLS verification (e.g. insecureSkipVerify). |
| operator.namespaces | string | `""` | Comma-separated list of namespaces the operator will watch. If empty, all namespaces are watched. |
| operator.nodeSelector | object | `{}` | Node label selector for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| operator.nodesetWorkers | int | `4` | Set the max concurrent workers for the NodeSet controller. |
| operator.pdb.enabled | bool | `false` | Enable PodDisruptionBudget. |
| operator.pdb.maxUnavailable | string | `nil` | Maximum pods that may be unavailable (int or quoted percent). Rendered only when set, and takes precedence over `minAvailable`. |
| operator.pdb.minAvailable | int | `1` | Minimum pods that must remain available after eviction (int or quoted percent). |
| operator.podSecurityContext | object | `{}` | Pod-level security context for the operator pod. Applied to all containers in the pod. Ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |
| operator.profile | bool | `false` | Enable Go profiling for slurm-operator |
| operator.profileAddr | string | `"localhost:6060"` | Set the port used for exposing Go profiling metrics. This should never be exposed on a public network. |
| operator.replicas | int | `1` | Set the number of replicas to deploy. |
| operator.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| operator.restapiWorkers | int | `4` | Set the max concurrent workers for the Restapi controller. |
| operator.securityContext | object | `{}` | Container-level security context for the operator container. Ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container |
| operator.serviceAccount.create | bool | `true` | Allows chart to create the service account. |
| operator.serviceAccount.name | string | `""` | Set the service account to use (and create). |
| operator.slurmclientWorkers | int | `2` | Set the max concurrent workers for the SlurmClient controller. |
| operator.tokenWorkers | int | `4` | Set the max concurrent workers for the Token controller. |
| operator.tolerations | list | `[]` | Tolerations for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| operator.topologySpreadConstraints | list | `[]` | Topology spread constraints for pod assignment. Prefer scheduling replicas across failure domains (nodes, zones, ...) when running in HA. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/ |
| priorityClassName | string | `""` | Set the priority class to use. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass |
| propagatedNodeConditions | list | `[]` | List of Kubernetes Node Conditions, by type, to propagate to the Slurm node drain reason. Ref: https://kubernetes.io/docs/reference/node/node-status/#condition |
| webhook.affinity | object | `{}` | Affinity for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| webhook.enabled | bool | `true` | Enable the webhook. |
| webhook.healthPort | int | `8081` | Set the port used for health checks. |
| webhook.image | object | `{"digest":null,"repository":"ghcr.io/slinkyproject/slurm-operator-webhook","tag":null}` | The image to use. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| webhook.imagePullPolicy | string | `"IfNotPresent"` | Set the image pull policy. |
| webhook.leaderElection | bool | `true` | Enable leader election for slurm-operator-webhook |
| webhook.logLevel | string | `"info"` | Set the log level by string (e.g. error, info, debug) or number (e.g. 1..5). |
| webhook.metricsPort | int | `0` | Set the port used by the metrics server. Value of "0" will disable it. |
| webhook.metricsSecure | bool | `false` | Serve the metrics endpoint securely via HTTPS with authn/authz. Requires metricsPort to be non-zero. Scraping clients must present a token authorized to access /metrics (e.g. bound to the metrics-reader ClusterRole). The endpoint uses a generated self-signed certificate, so scrapers must skip TLS verification (e.g. insecureSkipVerify). |
| webhook.mutating.failurePolicy | string | `"Ignore"` | Action taken when the mutating admission webhook is unreachable or returns an error. Ref: https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#failure-policy |
| webhook.mutating.matchPolicy | string | `"Equivalent"` | How the rules listed in the mutating webhook are matched against incoming requests. Ref: https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-matchpolicy |
| webhook.namespaces | string | `""` | Comma-separated list of namespaces the webhook will watch. If empty, all namespaces are watched. |
| webhook.nodeSelector | object | `{}` | Node label selector for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| webhook.pdb.enabled | bool | `false` | Enable PodDisruptionBudget. |
| webhook.pdb.maxUnavailable | string | `nil` | Maximum pods that may be unavailable (int or quoted percent). Rendered only when set, and takes precedence over `minAvailable`. |
| webhook.pdb.minAvailable | int | `1` | Minimum pods that must remain available after eviction (int or quoted percent). |
| webhook.podSecurityContext | object | `{}` | Pod-level security context for the webhook pod. Applied to all containers in the pod. Ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |
| webhook.replicas | int | `1` | Set the number of replicas to deploy. |
| webhook.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| webhook.securityContext | object | `{}` | Container-level security context for the webhook container. Ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container |
| webhook.serverPort | int | `9443` | Set the port used for the webhook server |
| webhook.serviceAccount.create | bool | `true` | Allows chart to create the service account. |
| webhook.serviceAccount.name | string | `""` | Set the service account to use (and create). |
| webhook.timeoutSeconds | int | `10` | Set the timeout period for calls. |
| webhook.tolerations | list | `[]` | Tolerations for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| webhook.topologySpreadConstraints | list | `[]` | Topology spread constraints for pod assignment. Prefer scheduling replicas across failure domains (nodes, zones, ...) when running in HA. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/ |
| webhook.validating.failurePolicy | string | `"Fail"` | Action taken when the validating admission webhook is unreachable or returns an error. Ref: https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#failure-policy |
| webhook.validating.matchPolicy | string | `"Equivalent"` | How the rules listed in the validating webhook are matched against incoming requests. Ref: https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-matchpolicy |

