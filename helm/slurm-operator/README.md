# slurm-operator

![Version: 0.4.0](https://img.shields.io/badge/Version-0.4.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 25.05](https://img.shields.io/badge/AppVersion-25.05-informational?style=flat-square)

Helm Chart for Slurm HPC Workload Manager Operator

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| file://../slurm-operator-crds | slurm-operator-crds | 0.4.0 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| certManager.duration | string | `"43800h0m0s"` | Duration of certificate life. |
| certManager.enabled | bool | `true` | Enable cert-manager for certificate management. |
| certManager.renewBefore | string | `"8760h0m0s"` | Certificate renewal time. Should be before the expiration. |
| certManager.secretName | string | `"slurm-operator-webhook-ca"` | The secret to be (created and) mounted. |
| crds | object | `{"enabled":false}` | Configure Custom Resource Definitions (CRDs). |
| crds.enabled | bool | `false` | Whether this helm chart should manage the CRD and its upgrades. |
| fullnameOverride | string | `""` | Overrides the full name of the release. |
| imagePullSecrets | list | `[]` | Sets the image pull secrets. Ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/ |
| nameOverride | string | `""` | Overrides the name of the release. |
| namespaceOverride | string | `""` | Overrides the namespace of the release. |
| operator.accountingWorkers | int | `4` | Set the max concurrent workers for the Accounting controller. |
| operator.affinity | object | `{}` | Affinity for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| operator.controllerWorkers | int | `4` | Set the max concurrent workers for the Controller controller. |
| operator.enabled | bool | `true` | Enables the operator. |
| operator.image | object | `{"repository":"ghcr.io/slinkyproject/slurm-operator","tag":""}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| operator.imagePullPolicy | string | `"IfNotPresent"` | Set the image pull policy. |
| operator.logLevel | string | `"info"` | Set the log level by string (e.g. error, info, debug) or number (e.g. 1..5). |
| operator.loginsetWorkers | int | `4` | Set the max concurrent workers for the LoginSet controller. |
| operator.nodesetWorkers | int | `4` | Set the max concurrent workers for the NodeSet controller. |
| operator.replicas | int | `1` | Set the number of replicas to deploy. |
| operator.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| operator.restapiWorkers | int | `4` | Set the max concurrent workers for the Restapi controller. |
| operator.serviceAccount.create | bool | `true` | Allows chart to create the service account. |
| operator.serviceAccount.name | string | `""` | Set the service account to use (and create). |
| operator.slurmclientWorkers | int | `2` | Set the max concurrent workers for the SlurmClient controller. |
| operator.tolerations | list | `[]` | Tolerations for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| priorityClassName | string | `""` | Set the priority class to use. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass |
| webhook.affinity | object | `{}` | Affinity for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| webhook.enabled | bool | `true` | Enable the webhook. |
| webhook.image | object | `{"repository":"ghcr.io/slinkyproject/slurm-operator-webhook","tag":""}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| webhook.imagePullPolicy | string | `"IfNotPresent"` | Set the image pull policy. |
| webhook.logLevel | string | `"info"` | Set the log level by string (e.g. error, info, debug) or number (e.g. 1..5). |
| webhook.replicas | int | `1` | Set the number of replicas to deploy. |
| webhook.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| webhook.serviceAccount.create | bool | `true` | Allows chart to create the service account. |
| webhook.serviceAccount.name | string | `""` | Set the service account to use (and create). |
| webhook.timeoutSeconds | int | `10` | Set the timeout period for calls. |
| webhook.tolerations | list | `[]` | Tolerations for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |

