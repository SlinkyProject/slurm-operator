# slurm

![Version: 0.4.0](https://img.shields.io/badge/Version-0.4.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 25.05](https://img.shields.io/badge/AppVersion-25.05-informational?style=flat-square)

Slurm Cluster

**Homepage:** <https://slurm.schedmd.com/>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| SchedMD LLC. | <slinky@schedmd.com> | <https://support.schedmd.com/> |

## Source Code

* <https://github.com/SchedMD/slurm>
* <https://github.com/SlinkyProject/containers>
* <https://github.com/SlinkyProject/slurm-operator>

## Requirements

Kubernetes: `>= 1.29.0-0`

| Repository | Name | Version |
|------------|------|---------|
| oci://ghcr.io/slinkyproject/charts | slurm-exporter | 0.4.0 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| accounting.enabled | bool | `false` | Enables Slurm accounting subsystem, stores job/step historical records. Ref: https://slurm.schedmd.com/accounting.html#Overview |
| accounting.extraConf | string | `nil` | Extra Slurm configuration lines appended to `slurmdbd.conf`. Ref: https://slurm.schedmd.com/slurmdbd.conf.html |
| accounting.extraConfMap | map[string]string \| map[string][]string | `{}` | Extra Slurm configuration lines appended to `slurmdbd.conf`. If `extraConf` is not empty, it takes precedence. Ref: https://slurm.schedmd.com/slurmdbd.conf.html |
| accounting.initconf.image | object | `{"repository":"ghcr.io/slinkyproject/sackd","tag":"25.05-ubuntu24.04"}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| accounting.initconf.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| accounting.metadata | object | `{}` | Labels and annotations. Ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ |
| accounting.podSpec | corev1.PodSpec | `{"affinity":{},"initContainers":[],"nodeSelector":{"kubernetes.io/os":"linux"},"tolerations":[]}` | Extend the pod template, and/or override certain configurations. Ref: https://kubernetes.io/docs/concepts/workloads/pods/#pod-templates |
| accounting.podSpec.affinity | object | `{}` | Affinity for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| accounting.podSpec.initContainers | list | `[]` | Additional initContainers for the pod. Ref: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/ Ref: https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/ |
| accounting.podSpec.nodeSelector | map[string]string | `{"kubernetes.io/os":"linux"}` | Node label selector for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| accounting.podSpec.tolerations | list | `[]` | Tolerations for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| accounting.service | object | `{}` | The service configuration. Ref: https://kubernetes.io/docs/concepts/services-networking/service/ |
| accounting.slurmdbd.args | list | `[]` | Arguments passed to the image. Ref: https://slurm.schedmd.com/slurmdbd.html#SECTION_OPTIONS |
| accounting.slurmdbd.image | object | `{"repository":"ghcr.io/slinkyproject/slurmdbd","tag":"25.05-ubuntu24.04"}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| accounting.slurmdbd.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| accounting.storageConfig.database | string | `"slurm_acct_db"` | The name of the database where records are written into. Ref: https://slurm.schedmd.com/slurmdbd.conf.html#OPT_StorageLoc |
| accounting.storageConfig.host | string | `"mariadb"` | The name of the host where the database is running. Ref: https://slurm.schedmd.com/slurmdbd.conf.html#OPT_StorageHost |
| accounting.storageConfig.passwordKeyRef | secretKeyRef | `{"key":"password","name":"mariadb-password"}` | The password used to connect to the database, from secret reference. Ref: https://slurm.schedmd.com/slurmdbd.conf.html#OPT_StoragePass |
| accounting.storageConfig.port | int | `3306` | The port number to communicate with the database with. Ref: https://slurm.schedmd.com/slurmdbd.conf.html#OPT_StoragePort |
| accounting.storageConfig.username | string | `"slurm"` | The name of the user used to connect to the database with. Ref: https://slurm.schedmd.com/slurmdbd.conf.html#OPT_StorageUser |
| clusterName | string | `nil` | The cluster name, which uniquely identifies the Slurm cluster. If empty, one will be derived from the Controller CR object. Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_ClusterName |
| configFiles | map[string]string | `{}` | Extra Slurm config files to be mounted to `/etc/slurm`. Ref: https://slurm.schedmd.com/man_index.html#configuration_files |
| controller.extraConf | string | `nil` | Extra Slurm configuration lines appended to `slurm.conf`. Ref: https://slurm.schedmd.com/slurm.conf.html |
| controller.extraConfMap | map[string]string \| map[string][]string | `{}` | Extra Slurm configuration lines appended to `slurm.conf`. If `extraConf` is not empty, it takes precedence. Ref: https://slurm.schedmd.com/slurm.conf.html |
| controller.logfile.image | object | `{"repository":"docker.io/library/alpine","tag":"latest"}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| controller.logfile.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| controller.metadata | object | `{}` | Labels and annotations. Ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ |
| controller.persistence.accessModes[0] | string | `"ReadWriteOnce"` |  |
| controller.persistence.enabled | bool | `true` | Enable persistence for slurmctld, retain save-state across recreations. |
| controller.persistence.existingClaim | string | `nil` | Name of the existing `PersistentVolumeClaim` to use instead of creating one. If this is not empty, then certain other fields will be ignored. |
| controller.persistence.resources | object | `{"requests":{"storage":"4Gi"}}` | The minimum resources for the `PersistentVolumeClaim` to be created with. Ref: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources |
| controller.persistence.storageClassName | string | `nil` | The name of the `StorageClass` for the created `PersistentVolumeClaim`. Ref: https://kubernetes.io/docs/concepts/storage/storage-classes/ |
| controller.podSpec | corev1.PodSpec | `{"affinity":{},"initContainers":[],"nodeSelector":{"kubernetes.io/os":"linux"},"tolerations":[]}` | Extend the pod template, and/or override certain configurations. Ref: https://kubernetes.io/docs/concepts/workloads/pods/#pod-templates |
| controller.podSpec.affinity | object | `{}` | Affinity for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| controller.podSpec.initContainers | list | `[]` | Additional initContainers for the pod. Ref: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/ Ref: https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/ |
| controller.podSpec.nodeSelector | map[string]string | `{"kubernetes.io/os":"linux"}` | Node label selector for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| controller.podSpec.tolerations | list | `[]` | Tolerations for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| controller.reconfigure.image | object | `{"repository":"ghcr.io/slinkyproject/slurmctld","tag":"25.05-ubuntu24.04"}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| controller.reconfigure.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| controller.service | object | `{}` | The service configuration. Ref: https://kubernetes.io/docs/concepts/services-networking/service/ |
| controller.slurmctld.args | list | `[]` | Arguments passed to the image. Ref: https://slurm.schedmd.com/slurmctld.html#SECTION_OPTIONS |
| controller.slurmctld.image | object | `{"repository":"ghcr.io/slinkyproject/slurmctld","tag":"25.05-ubuntu24.04"}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| controller.slurmctld.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| epilogScripts | map[string]string | `{}` | The Slurm Epilog scripts ran on all NodeSets. The map key represents the filename; the map value represents the script contents. WARNING: The script must include a shebang (!) so it can be executed correctly by Slurm. Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_Epilog Ref: https://slurm.schedmd.com/prolog_epilog.html Ref: https://en.wikipedia.org/wiki/Shebang_(Unix) |
| epilogSlurmctldScripts | map[string]string | `{}` | The Slurm EpilogSlurmctld scripts ran on slurmctld at job completion. The map key represents the filename; the map value represents the script contents. WARNING: The script must include a shebang (!) so it can be executed correctly by Slurm. Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_EpilogSlurmctld Ref: https://slurm.schedmd.com/prolog_epilog.html Ref: https://en.wikipedia.org/wiki/Shebang_(Unix) |
| fullnameOverride | string | `nil` | Overrides the full name of the release. |
| imagePullPolicy | string | `"IfNotPresent"` | Set the image pull policy. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy |
| imagePullSecrets | list | `[]` | Set the secrets for image pull. Ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/ |
| jwtHs256KeyRef | secretKeyRef | `{}` | Slurm cluster JWT HS256 authentication key. If empty, one will be generated and used. Ref: https://slurm.schedmd.com/authentication.html#jwt |
| loginsets.slinky.enabled | bool | `false` | Enable use of this LoginSet. |
| loginsets.slinky.extraSshdConfig | string | `nil` | Extra configuration lines appended to `/etc/ssh/sshd_config`. Ref: https://manpages.ubuntu.com/manpages/noble/man5/sshd_config.5.html |
| loginsets.slinky.login.env | list | `[]` | Environment passed to the image. |
| loginsets.slinky.login.image | object | `{"repository":"ghcr.io/slinkyproject/login","tag":"25.05-ubuntu24.04"}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| loginsets.slinky.login.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| loginsets.slinky.login.securityContext | object | `{"privileged":false}` | The container security context to use. Ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container |
| loginsets.slinky.login.volumeMounts | list | `[]` | List of volume mounts to use. Ref: https://kubernetes.io/docs/concepts/storage/volumes/ |
| loginsets.slinky.metadata | object | `{}` | Labels and annotations. Ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ |
| loginsets.slinky.podSpec | corev1.PodSpec | `{"affinity":{},"initContainers":[],"nodeSelector":{"kubernetes.io/os":"linux"},"tolerations":[],"volumes":[]}` | Extend the pod template, and/or override certain configurations. Ref: https://kubernetes.io/docs/concepts/workloads/pods/#pod-templates |
| loginsets.slinky.podSpec.affinity | object | `{}` | Affinity for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| loginsets.slinky.podSpec.initContainers | list | `[]` | Additional initContainers for the pod. Ref: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/ Ref: https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/ |
| loginsets.slinky.podSpec.nodeSelector | map[string]string | `{"kubernetes.io/os":"linux"}` | Node label selector for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| loginsets.slinky.podSpec.tolerations | list | `[]` | Tolerations for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| loginsets.slinky.podSpec.volumes | list | `[]` | List of volumes to use. Ref: https://kubernetes.io/docs/concepts/storage/volumes/ |
| loginsets.slinky.replicas | int | `1` | Number of replicas to deploy. |
| loginsets.slinky.rootSshAuthorizedKeys | string | `nil` | SSH public keys to write into `/root/.ssh/authorized_keys`. |
| loginsets.slinky.service | object | `{"spec":{"type":"LoadBalancer"}}` | The service configuration. Ref: https://kubernetes.io/docs/concepts/services-networking/service/ |
| loginsets.slinky.sssdConf | string | `"[sssd]\nconfig_file_version = 2\nservices = nss,pam\ndomains = DEFAULT\n\n[nss]\nfilter_groups = root,slurm\nfilter_users = root,slurm\n\n[pam]\n\n[domain/DEFAULT]\nauth_provider = ldap\nid_provider = ldap\nldap_uri = ldap://ldap.example.com\nldap_search_base = dc=example,dc=com\nldap_user_search_base = ou=Users,dc=example,dc=com\nldap_group_search_base = ou=Groups,dc=example,dc=com\n"` | The `sssd.conf` to use. Ref: https://man.archlinux.org/man/sssd.conf.5 |
| nameOverride | string | `nil` | Overrides the name of the release. |
| namespaceOverride | string | `nil` | Overrides the namespace of the release. |
| nodesets.slinky.enabled | bool | `true` | Enable use of this NodeSet. |
| nodesets.slinky.extraConf | string | `nil` | Extra configuration added to the `--conf` argument. Ref: https://slurm.schedmd.com/slurm.conf.html#SECTION_NODE-CONFIGURATION |
| nodesets.slinky.extraConfMap | map[string]string \| map[string][]string | `{}` | Extra configuration added to the `--conf` argument. If `extraConf` is not empty, it takes precedence. Ref: https://slurm.schedmd.com/slurm.conf.html#SECTION_NODE-CONFIGURATION |
| nodesets.slinky.logfile.image | object | `{"repository":"docker.io/library/alpine","tag":"latest"}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| nodesets.slinky.logfile.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| nodesets.slinky.metadata | object | `{}` | Labels and annotations. Ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ |
| nodesets.slinky.partition.config | string | `nil` | The Slurm partition configuration options added to the partition line added to the partition line. Ref: https://slurm.schedmd.com/slurm.conf.html#SECTION_PARTITION-CONFIGURATION |
| nodesets.slinky.partition.configMap | map[string]string \| map[string][]string | `{}` | The Slurm partition configuration options added to the partition line. If `config` is not empty, it takes precedence. Ref: https://slurm.schedmd.com/slurm.conf.html#SECTION_PARTITION-CONFIGURATION |
| nodesets.slinky.partition.enabled | bool | `true` | Enable NodeSet partition creation. |
| nodesets.slinky.podSpec | corev1.PodSpec | `{"affinity":{},"initContainers":[],"nodeSelector":{"kubernetes.io/os":"linux"},"tolerations":[],"volumes":[]}` | Extend the pod template, and/or override certain configurations. Ref: https://kubernetes.io/docs/concepts/workloads/pods/#pod-templates |
| nodesets.slinky.podSpec.affinity | object | `{}` | Affinity for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| nodesets.slinky.podSpec.initContainers | list | `[]` | Additional initContainers for the pod. Ref: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/ Ref: https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/ |
| nodesets.slinky.podSpec.nodeSelector | map[string]string | `{"kubernetes.io/os":"linux"}` | Node label selector for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| nodesets.slinky.podSpec.tolerations | list | `[]` | Tolerations for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| nodesets.slinky.podSpec.volumes | list | `[]` | List of volumes to use. Ref: https://kubernetes.io/docs/concepts/storage/volumes/ |
| nodesets.slinky.replicas | int | `1` | Number of replicas to deploy. |
| nodesets.slinky.slurmd.args | list | `[]` | Arguments passed to the image. Ref: https://slurm.schedmd.com/slurmd.html#SECTION_OPTIONS |
| nodesets.slinky.slurmd.image | object | `{"repository":"ghcr.io/slinkyproject/slurmd","tag":"25.05-ubuntu24.04"}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| nodesets.slinky.slurmd.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| nodesets.slinky.slurmd.volumeMounts | list | `[]` | List of volume mounts to use. Ref: https://kubernetes.io/docs/concepts/storage/volumes/ |
| nodesets.slinky.updateStrategy | object | `{"rollingUpdate":{"maxUnavailable":"25%"},"type":"RollingUpdate"}` | Update strategy for rolling updates. Ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#update-strategies |
| nodesets.slinky.updateStrategy.rollingUpdate.maxUnavailable | string | `"25%"` | Maximum number of pods that can be unavailable during update. Can be an absolute number (ex: 5) or a percentage (ex: 25%). |
| nodesets.slinky.useResourceLimits | bool | `true` | Enable propagation of container `resources.limits` into slurmd. |
| partitions.all.config | string | `nil` | The Slurm partition configuration options added to the partition line. Ref: https://slurm.schedmd.com/slurm.conf.html#SECTION_PARTITION-CONFIGURATION |
| partitions.all.configMap | map[string]string \| map[string][]string | `{"Default":"YES","MaxTime":"UNLIMITED","State":"UP"}` | The Slurm partition configuration options added to the partition line. If `config` is not empty, it takes precedence. Ref: https://slurm.schedmd.com/slurm.conf.html#SECTION_PARTITION-CONFIGURATION |
| partitions.all.enabled | bool | `true` | Enable use of this partition. |
| partitions.all.nodesets | list | `["ALL"]` | NodeSets to associate with this partition. NOTE: NodeSet "ALL" is mapped to all NodeSet configured in the cluster. |
| priorityClassName | string | `nil` | Set the priority class to use. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass |
| prologScripts | map[string]string | `{}` | The Slurm Prolog scripts ran on all NodeSets. The map key represents the filename; the map value represents the script contents. WARNING: The script must include a shebang (!) so it can be executed correctly by Slurm. Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_Prolog Ref: https://slurm.schedmd.com/prolog_epilog.html Ref: https://en.wikipedia.org/wiki/Shebang_(Unix) |
| prologSlurmctldScripts | map[string]string | `{}` | The Slurm PrologSlurmctld scripts run on slurmctld at job allocation. The map key represents the filename; the map value represents the script contents. WARNING: The script must include a shebang (!) so it can be executed correctly by Slurm. Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_PrologSlurmctld Ref: https://slurm.schedmd.com/prolog_epilog.html Ref: https://en.wikipedia.org/wiki/Shebang_(Unix) |
| restapi.metadata | object | `{}` | Labels and annotations. Ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ |
| restapi.podSpec | corev1.PodSpec | `{"affinity":{},"initContainers":[],"nodeSelector":{"kubernetes.io/os":"linux"},"tolerations":[]}` | Extend the pod template, and/or override certain configurations. Ref: https://kubernetes.io/docs/concepts/workloads/pods/#pod-templates |
| restapi.podSpec.affinity | object | `{}` | Affinity for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| restapi.podSpec.initContainers | list | `[]` | Additional initContainers for the pod. Ref: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/ Ref: https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/ |
| restapi.podSpec.nodeSelector | map[string]string | `{"kubernetes.io/os":"linux"}` | Node label selector for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| restapi.podSpec.tolerations | list | `[]` | Tolerations for pod assignment. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| restapi.replicas | int | `1` | Number of replicas to deploy. |
| restapi.service | object | `{}` | The service configuration. Ref: https://kubernetes.io/docs/concepts/services-networking/service/ |
| restapi.slurmrestd.args | list | `[]` | Arguments passed to the image. Ref: https://slurm.schedmd.com/slurmrestd.html#SECTION_OPTIONS |
| restapi.slurmrestd.env | list | `[]` | Environment passed to the image. Ref: https://slurm.schedmd.com/slurmrestd.html#SECTION_ENVIRONMENT-VARIABLES |
| restapi.slurmrestd.image | object | `{"repository":"ghcr.io/slinkyproject/slurmrestd","tag":"25.05-ubuntu24.04"}` | The image to use, `${repository}:${tag}`. Ref: https://kubernetes.io/docs/concepts/containers/images/#image-names |
| restapi.slurmrestd.resources | object | `{}` | The container resource limits and requests. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| slurm-exporter.enabled | bool | `true` |  |
| slurm-exporter.exporter.affinity | object | `{}` |  |
| slurm-exporter.exporter.enabled | bool | `true` |  |
| slurm-exporter.exporter.nodeSelector."kubernetes.io/os" | string | `"linux"` |  |
| slurm-exporter.exporter.secretName | string | `"slurm-token-exporter"` |  |
| slurm-exporter.exporter.tolerations | list | `[]` |  |
| slurmKeyRef | secretKeyRef | `{}` | Slurm shared authentication key. If empty, one will be generated and used. Ref: https://slurm.schedmd.com/authentication.html#slurm |
| vendor.nvidia.dcgm.enabled | bool | `false` | Enable DCGM GPU-to-job mapping integration |
| vendor.nvidia.dcgm.jobMappingDir | string | `"/var/lib/dcgm-exporter/job-mapping"` | Directory path where GPU-to-job mapping files will be stored |
| vendor.nvidia.dcgm.scriptPriority | string | `"90"` | Script execution priority (lower numbers run first) |

