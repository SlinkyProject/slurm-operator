# slurm

![Version: 0.4.0](https://img.shields.io/badge/Version-0.4.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 25.05](https://img.shields.io/badge/AppVersion-25.05-informational?style=flat-square)

Helm Chart for Slurm HPC Workload Manager

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| oci://ghcr.io/slinkyproject/charts | slurm-exporter | ~0.3.0 |
| oci://registry-1.docker.io/bitnamicharts | mariadb | ~20.4 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| accounting.affinity | object | `{}` |  Set affinity for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| accounting.enabled | bool | `true` |  Enables accounting services. |
| accounting.external.database | string | `"slurm_acct_db"` |  The database context to use. |
| accounting.external.enabled | bool | `false` |  Use an external database instead of creating one. WARNING: `accounting.external.enabled=true` is mutually exclusive with `mariadb.enabled=true`. |
| accounting.external.host | string | `""` |  The host or service to communicate with. |
| accounting.external.password | string | `""` |  The plaintext user password to the database. NOTE: ignored when secretName is not empty. |
| accounting.external.port | integer | `3306` |  The database port to use. |
| accounting.external.secretName | string | `""` |  The existing secret containing the user password to the database. |
| accounting.external.user | string | `"slurm"` |  The database user to use. |
| accounting.image.repository | string | `"ghcr.io/slinkyproject/slurmdbd"` |  Set the image repository to use. |
| accounting.image.tag | string | `"25.05-ubuntu24.04"` |  Set the image tag to use. |
| accounting.imagePullPolicy | string | `"IfNotPresent"` |  Set the image pull policy. |
| accounting.nodeSelector | map | `{"kubernetes.io/os":"linux"}` |  Selector which must match a node's labels for the pod to be scheduled on that node. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| accounting.resources | object | `{}` |  Set container resource requests and limits for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| accounting.tolerations | list | `[]` |  Configure pod tolerations. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| authcred.image.repository | string | `"ghcr.io/slinkyproject/sackd"` |  Set the image repository to use. |
| authcred.image.tag | string | `"25.05-ubuntu24.04"` |  Set the image tag to use. |
| authcred.imagePullPolicy | string | `"IfNotPresent"` |  Set the image pull policy. |
| authcred.resources | object | `{}` |  Set container resource requests and limits for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| compute.image.repository | string | `"ghcr.io/slinkyproject/slurmd"` |  Set the image repository to use. |
| compute.image.tag | string | `"25.05-ubuntu24.04"` |  Set the image tag to use. |
| compute.imagePullPolicy | string | `"IfNotPresent"` |  Set the image pull policy. |
| compute.nodesets | list | `[{"affinity":{},"authcred":{},"enabled":true,"extraVolumeMounts":[],"extraVolumes":[],"image":{"repository":"","tag":""},"imagePullPolicy":"IfNotPresent","name":"debug","nodeConfig":{},"nodeSelector":{"kubernetes.io/os":"linux"},"partition":{"config":{"MaxTime":"UNLIMITED","State":"UP"},"enabled":true},"persistentVolumeClaimRetentionPolicy":{"whenDeleted":"Retain","whenScaled":"Retain"},"priorityClassName":"","replicas":1,"resources":{},"tolerations":[],"updateStrategy":{"rollingUpdate":{"maxUnavailable":"20%"},"type":"RollingUpdate"},"useResourceLimits":true,"volumeClaimTemplates":[]}]` |  Slurm NodeSets by object list. |
| compute.nodesets[0] | string | `{"affinity":{},"authcred":{},"enabled":true,"extraVolumeMounts":[],"extraVolumes":[],"image":{"repository":"","tag":""},"imagePullPolicy":"IfNotPresent","name":"debug","nodeConfig":{},"nodeSelector":{"kubernetes.io/os":"linux"},"partition":{"config":{"MaxTime":"UNLIMITED","State":"UP"},"enabled":true},"persistentVolumeClaimRetentionPolicy":{"whenDeleted":"Retain","whenScaled":"Retain"},"priorityClassName":"","replicas":1,"resources":{},"tolerations":[],"updateStrategy":{"rollingUpdate":{"maxUnavailable":"20%"},"type":"RollingUpdate"},"useResourceLimits":true,"volumeClaimTemplates":[]}` |  Name of NodeSet. Must be unique. |
| compute.nodesets[0].affinity | object | `{}` |  Set affinity for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| compute.nodesets[0].authcred | object | `{}` |  NodeSet specific authcred configuration, otherwise global authcred is used. |
| compute.nodesets[0].enabled | bool | `true` |  Enables the NodeSet in Slurm. |
| compute.nodesets[0].extraVolumeMounts | list | `[]` |  List of volume mounts. Ref: https://kubernetes.io/docs/concepts/storage/volumes/ |
| compute.nodesets[0].extraVolumes | list | `[]` |  Define list of pod volumes. Ref: https://kubernetes.io/docs/concepts/storage/volumes/ |
| compute.nodesets[0].image.repository | string | `""` |  Set the image repository to use. |
| compute.nodesets[0].image.tag | string | `""` |  Set the image tag to use. |
| compute.nodesets[0].imagePullPolicy | string | `"IfNotPresent"` |  Set the image pull policy. |
| compute.nodesets[0].nodeConfig | map[string]string || map[string][]string | `{}` |  Extra Slurm node configuration appended into the --conf arguments. WARNING: Values can override existing ones. Ref: https://slurm.schedmd.com/slurm.conf.html#lbAE |
| compute.nodesets[0].nodeSelector | map | `{"kubernetes.io/os":"linux"}` |  Selector which must match a node's labels for the pod to be scheduled on that node. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| compute.nodesets[0].partition | object | `{"config":{"MaxTime":"UNLIMITED","State":"UP"},"enabled":true}` |  Partition describes the partition created specifically for this NodeSet to be added. |
| compute.nodesets[0].partition.config | map[string]string | map[string][]string | `{"MaxTime":"UNLIMITED","State":"UP"}` |  Extra Slurm partition configuration appended onto the partition line. Ref: https://slurm.schedmd.com/slurm.conf.html#lbAI |
| compute.nodesets[0].partition.enabled | bool | `true` |  Enables this NodeSet's partition line to be added in Slurm. |
| compute.nodesets[0].persistentVolumeClaimRetentionPolicy | object | `{"whenDeleted":"Retain","whenScaled":"Retain"}` |  The policy used for PVCs created from the NodeSet VolumeClaimTemplates. |
| compute.nodesets[0].persistentVolumeClaimRetentionPolicy.whenDeleted | string | `"Retain"` |  WhenDeleted specifies what happens to PVCs created from NodeSet VolumeClaimTemplates when the NodeSet is deleted. The default policy of `Retain` causes PVCs to not be affected by NodeSet deletion. The `Delete` policy causes those PVCs to be deleted. |
| compute.nodesets[0].persistentVolumeClaimRetentionPolicy.whenScaled | string | `"Retain"` |  WhenScaled specifies what happens to PVCs created from NodeSet VolumeClaimTemplates when the NodeSet is scaled down. The default policy of `Retain` causes PVCs to not be affected by a scale-in. The `Delete` policy causes the associated PVCs for any excess pods to be deleted. |
| compute.nodesets[0].priorityClassName | string | `""` |  Set the priority class to use. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass |
| compute.nodesets[0].replicas | integer | `1` |  Set the number of replicas to deploy. |
| compute.nodesets[0].resources | object | `{}` |  Set container resource requests and limits for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| compute.nodesets[0].tolerations | list | `[]` |  Configure pod tolerations. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| compute.nodesets[0].updateStrategy | object | `{"rollingUpdate":{"maxUnavailable":"20%"},"type":"RollingUpdate"}` |  Set the update strategy configuration. |
| compute.nodesets[0].updateStrategy.rollingUpdate | object | `{"maxUnavailable":"20%"}` |  Define the rolling update policy. Only used when "updateStrategy.type=RollingUpdate". |
| compute.nodesets[0].updateStrategy.rollingUpdate.maxUnavailable | string | `"20%"` |  The maximum number of pods that can be unavailable during the update. Value can be an absolute number (ex: 5) or a percentage of desired pods (ex: 10%). Absolute number is calculated from percentage by rounding up. This can not be 0. Defaults to 1. |
| compute.nodesets[0].updateStrategy.type | string | `"RollingUpdate"` |  Set the update strategy type. Can be either: "RollingUpdate"; "OnDelete". |
| compute.nodesets[0].useResourceLimits | bool | `true` |  Enable to propagate the pod `resources.limits` into slurmd. |
| compute.nodesets[0].volumeClaimTemplates | list | `[]` |  List of PVCs to be created from template and mounted on each NodeSet pod. PVCs are given a unique identity relative to each NodeSet pod. Ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#volume-claim-templates |
| compute.partitions | list | `[{"config":{"Default":"YES","MaxTime":"UNLIMITED","State":"UP"},"enabled":true,"name":"all","nodesets":["ALL"]}]` |  Slurm Partitions by object list. |
| compute.partitions[0] | string | `{"config":{"Default":"YES","MaxTime":"UNLIMITED","State":"UP"},"enabled":true,"name":"all","nodesets":["ALL"]}` |  Name of Partition. Must be unique. |
| compute.partitions[0].config | map[string]string | map[string][]string | `{"Default":"YES","MaxTime":"UNLIMITED","State":"UP"}` |  Extra Slurm partition configuration appended onto the partition line. Ref: https://slurm.schedmd.com/slurm.conf.html#lbAI |
| compute.partitions[0].enabled | bool | `true` |  Enables the partition in Slurm. |
| compute.partitions[0].nodesets | list | `["ALL"]` |  NodeSets to put into this Partition by name/key. NOTE: 'ALL' is a Slurm meta value to mean all nodes in the system. |
| controller.affinity | object | `{}` |  Set affinity for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| controller.image.repository | string | `"ghcr.io/slinkyproject/slurmctld"` |  Set the image repository to use. |
| controller.image.tag | string | `"25.05-ubuntu24.04"` |  Set the image tag to use. |
| controller.imagePullPolicy | string | `"IfNotPresent"` |  Set the image pull policy. |
| controller.nodeSelector | map | `{"kubernetes.io/os":"linux"}` |  Selector which must match a node's labels for the pod to be scheduled on that node. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| controller.persistence.accessModes | list | `["ReadWriteOnce"]` |  Create a `PersistentVolumeClaim` with these access modes. |
| controller.persistence.annotations | object | `{}` |  Create a `PersistentVolumeClaim` with these annotations. |
| controller.persistence.enabled | bool | `true` |  Enables save-state persistence. |
| controller.persistence.existingClaim | string | `""` |  Name of an existing `PersistentVolumeClaim` to use instead of creating one from definition. NOTE: When not empty, the other persistence fields will be ignored. |
| controller.persistence.labels | object | `{}` |  Create a `PersistentVolumeClaim` with these labels. |
| controller.persistence.selector | object | `{}` |  Selector to match an existing `PersistentVolume`. |
| controller.persistence.size | string | `"4Gi"` |  Create a `PersistentVolumeClaim` with this storage size. |
| controller.persistence.storageClass | string | `nil` |  Create a `PersistentVolumeClaim` with this storage class. |
| controller.priorityClassName | string | `""` |  Set the priority class to use. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass |
| controller.resources | object | `{}` |  Set container resource requests and limits for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| controller.service | object | `{}` |  The controller service configuration. Ref: https://kubernetes.io/docs/concepts/services-networking/service/ |
| controller.serviceNodePort | integer | `36817` |  The external service node port number. Ignored unless `service.type == NodePort`. |
| controller.servicePort | integer | `6817` |  The external service port number. |
| controller.tolerations | list | `[]` |  Configure pod tolerations. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| fullnameOverride | string | `""` |  Overrides the full name of the release. |
| imagePullPolicy | string | `"IfNotPresent"` |  Set the image pull policy. |
| imagePullSecrets | list | `[]` |  Set the secrets for image pull. Ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/ |
| jwt.hs256.secretName | string | `""` |  The existing secret containing the jwt_hs256.key, otherwise one will be generated. |
| login.affinity | object | `{}` |  Set affinity for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| login.enabled | bool | `false` |  Enables login nodes. |
| login.extraVolumeMounts | list | `[]` |  List of volume mounts. Ref: https://kubernetes.io/docs/concepts/storage/volumes/ |
| login.extraVolumes | list | `[]` |  Define list of pod volumes. Ref: https://kubernetes.io/docs/concepts/storage/volumes/ |
| login.image.repository | string | `"ghcr.io/slinkyproject/login"` |  Set the image repository to use. |
| login.image.tag | string | `"25.05-ubuntu24.04"` |  Set the image tag to use. |
| login.imagePullPolicy | string | `"IfNotPresent"` |  Set the image pull policy. |
| login.nodeSelector | map | `{"kubernetes.io/os":"linux"}` |  Selector which must match a node's labels for the pod to be scheduled on that node. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| login.priorityClassName | string | `""` |  Set the priority class to use. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass |
| login.replicas | integer | `1` |  Set the number of replicas to deploy. |
| login.resources | object | `{}` |  Set container resource requests and limits for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| login.rootSshAuthorizedKeys | list | `[]` |  The `/root/.ssh/authorized_keys` file to write, represented as a list. |
| login.securityContext | object | `{"privileged":false}` |  The security context given to the container. Ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container |
| login.service | object | `{"type":"LoadBalancer"}` |  The login service configuration. Ref: https://kubernetes.io/docs/concepts/services-networking/service/ |
| login.serviceNodePort | integer | `32222` |  The external service node port number. Ignored unless `service.type == NodePort`. |
| login.servicePort | integer | `2222` |  The external service port number. |
| login.sshdConfig | map | `{"Include":"/etc/ssh/sshd_config.d/*.conf","Subsystem":"sftp /usr/libexec/openssh/sftp-server","UsePAM":"yes","X11Forwarding":"yes"}` |  The `/etc/ssh/sshd_config` file to use, represented as a map. Ref: https://man.openbsd.org/sshd_config |
| login.sssdConf.domains | map[map] | `{"DEFAULT":{"auth_provider":"ldap","id_provider":"ldap","ldap_group_search_base":"ou=Groups,dc=example,dc=com","ldap_search_base":"dc=example,dc=com","ldap_uri":"ldap://ldap.example.com","ldap_user_search_base":"ou=Users,dc=example,dc=com"}}` |  The `/etc/sssd/sssd.conf` [domain/$DOMAIN] sections, represented as a map of map. Ref: https://man.archlinux.org/man/sssd.conf.5#DOMAIN_SECTIONS |
| login.sssdConf.nss | map | `{"filter_groups":"root,slurm","filter_users":"root,slurm"}` |  The `/etc/sssd/sssd.conf` [nss] section, represented as a map. Ref: https://man.archlinux.org/man/sssd.conf.5#NSS_configuration_options |
| login.sssdConf.pam | map | `{}` |  The `/etc/sssd/sssd.conf` [pam] section, represented as a map. Ref: https://man.archlinux.org/man/sssd.conf.5#PAM_configuration_options |
| login.sssdConf.sssd | map | `{"config_file_version":2,"domains":"DEFAULT","services":"nss, pam"}` |  The `/etc/sssd/sssd.conf` [sssd] section, represented as a map. Ref: https://man.archlinux.org/man/sssd.conf.5#The_%5Bsssd%5D_section |
| login.tolerations | list | `[]` |  Configure pod tolerations. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| mariadb.affinity | object | `{}` |  |
| mariadb.auth.database | string | `"slurm_acct_db"` |  |
| mariadb.auth.username | string | `"slurm"` |  |
| mariadb.enabled | bool | `true` |  |
| mariadb.metrics.enabled | bool | `false` |  |
| mariadb.metrics.serviceMonitor.enabled | bool | `false` |  |
| mariadb.nodeSelector."kubernetes.io/os" | string | `"linux"` |  |
| mariadb.primary.configuration | string | `"[mysqld]\nskip-name-resolve\nexplicit_defaults_for_timestamp\nbasedir=/opt/bitnami/mariadb\ndatadir=/bitnami/mariadb/data\nplugin_dir=/opt/bitnami/mariadb/plugin\nport={{ .Values.primary.containerPorts.mysql }}\nsocket=/opt/bitnami/mariadb/tmp/mysql.sock\ntmpdir=/opt/bitnami/mariadb/tmp\ninnodb_buffer_pool_size=4096M\ninnodb_lock_wait_timeout=900\ninnodb_log_file_size=1024M\nmax_allowed_packet=16M\nbind-address=*\npid-file=/opt/bitnami/mariadb/tmp/mysqld.pid\nlog-error=/opt/bitnami/mariadb/logs/mysqld.log\ncharacter-set-server=UTF8\ncollation-server=utf8_general_ci\nslow_query_log=0\nlong_query_time=10.0\nbinlog_expire_logs_seconds=2592000\n{{- if .Values.tls.enabled }}\nssl_cert=/opt/bitnami/mariadb/certs/{{ .Values.tls.certFilename }}\nssl_key=/opt/bitnami/mariadb/certs/{{ .Values.tls.certKeyFilename }}\n{{- if (include \"mariadb.tlsCACert\" .) }}\nssl_ca={{ include \"mariadb.tlsCACert\" . }}\n{{- end }}\n{{- end }}\n{{- if .Values.tde.enabled }}\nplugin_load_add=file_key_management\nfile_key_management_filename=/opt/bitnami/mariadb/tde/{{ .Values.tde.encryptedKeyFilename }}\nfile_key_management_filekey=FILE:/opt/bitnami/mariadb/tde/{{ .Values.tde.randomKeyFilename }}\nfile_key_management_encryption_algorithm={{ .Values.tde.fileKeyManagementEncryptionAlgorithm }}\ninnodb_encrypt_tables={{ .Values.tde.innodbEncryptTables }}\ninnodb_encrypt_log={{ .Values.tde.innodbEncryptLog }}\ninnodb_encrypt_temporary_tables={{ .Values.tde.innodbEncryptTemporaryTables }}\ninnodb_encryption_threads={{ .Values.tde.innodbEncryptionThreads }}\nencrypt_tmp_disk_tables={{ .Values.tde.encryptTmpDiskTables }}\nencrypt_tmp_files={{ .Values.tde.encryptTmpTiles }}\nencrypt_binlog={{ .Values.tde.encryptBINLOG }}\naria_encrypt_tables={{ .Values.tde.ariaEncryptTables }}\n{{- end }}\n\n[client]\nport=3306\nsocket=/opt/bitnami/mariadb/tmp/mysql.sock\ndefault-character-set=UTF8\nplugin_dir=/opt/bitnami/mariadb/plugin\n\n[manager]\nport=3306\nsocket=/opt/bitnami/mariadb/tmp/mysql.sock\npid-file=/opt/bitnami/mariadb/tmp/mysqld.pid"` |  |
| mariadb.primary.persistence.accessModes[0] | string | `"ReadWriteOnce"` |  |
| mariadb.primary.persistence.annotations | object | `{}` |  |
| mariadb.primary.persistence.enabled | bool | `true` |  |
| mariadb.primary.persistence.existingClaim | string | `""` |  |
| mariadb.primary.persistence.labels | object | `{}` |  |
| mariadb.primary.persistence.selector | object | `{}` |  |
| mariadb.primary.persistence.size | string | `"8Gi"` |  |
| mariadb.primary.persistence.storageClass | string | `nil` |  |
| mariadb.primary.priorityClassName | string | `""` |  |
| mariadb.primary.tolerations | list | `[]` |  |
| mariadb.resources | object | `{}` |  |
| mariadb.tde.enabled | bool | `false` |  |
| mariadb.tls.enabled | bool | `false` |  |
| nameOverride | string | `""` |  Overrides the name of the release. |
| namespaceOverride | string | `""` |  Overrides the namespace of the release. |
| priorityClassName | string | `""` |  Set the priority class to use. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass |
| restapi.affinity | object | `{}` |  Set affinity for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity |
| restapi.enabled | bool | `true` |  Enables restapi services. |
| restapi.image.repository | string | `"ghcr.io/slinkyproject/slurmrestd"` |  Set the image repository to use. |
| restapi.image.tag | string | `"25.05-ubuntu24.04"` |  Set the image tag to use. |
| restapi.imagePullPolicy | string | `"IfNotPresent"` |  Set the image pull policy. |
| restapi.nodeSelector | map | `{"kubernetes.io/os":"linux"}` |  Selector which must match a node's labels for the pod to be scheduled on that node. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector |
| restapi.priorityClassName | string | `""` |  Set the priority class to use. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass |
| restapi.replicas | integer | `1` |  Set the number of replicas to deploy. |
| restapi.resources | object | `{}` |  Set container resource requests and limits for Kubernetes Pod scheduling. Ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-requests-and-limits-of-pod-and-container |
| restapi.service | object | `{}` |  The restapi service configuration. Ref: https://kubernetes.io/docs/concepts/services-networking/service/ |
| restapi.serviceNodePort | integer | `36820` |  The external service node port number. Ignored unless `service.type == NodePort`. |
| restapi.servicePort | integer | `6820` |  The external service port number. |
| restapi.tolerations | list | `[]` |  Configure pod tolerations. Ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| slurm-exporter.enabled | bool | `true` |  |
| slurm-exporter.exporter.affinity | object | `{}` |  |
| slurm-exporter.exporter.enabled | bool | `true` |  |
| slurm-exporter.exporter.secretName | string | `"slurm-token-exporter"` |  |
| slurm.auth.secretName | string | `""` |  The existing secret containing the slurm.key, otherwise one will be generated. |
| slurm.configFiles | map[string]string | `{"cgroup.conf":"CgroupPlugin=autodetect\nIgnoreSystemd=yes\nEnableControllers=yes\nConstrainCores=yes\nConstrainRAMSpace=yes\nConstrainDevices=yes\nConstrainSwapSpace=yes\n","slurm_extra.conf":"# Core defaults (moved from template hardcoded values)\nGresTypes=gpu\nReturnToService=2\nMaxNodeCount=1024\nLogTimeFormat=iso8601_ms,format_stderr\nSelectTypeParameters=CR_Core_Memory\nCommunicationParameters=block_null_hash\nLaunchParameters=enable_nss_slurm,use_interactive_step,ulimit_pam_adopt\nJobAcctGatherFrequency=task=30\nAccountingStorageTRES=gres/gpu\n# User customizations\n# MinJobAge=2\n### LOGGING ###\n# SlurmctldDebug=debug2\n# SlurmSchedLogLevel=1\n# SlurmdDebug=debug2\n# DebugFlags=\n### PLUGINS & PARAMETERS ###\n# SchedulerParameters=defer_batch\n","slurmdbd_extra.conf":"# Core defaults (moved from template hardcoded values)\nLogFile=/dev/null\n# User customizations\n# CommitDelay=1\n### LOGGING ###\n# DebugLevel=debug2\n# DebugFlags=\n### PURGE ###\n# PurgeEventAfter=12month\n# PurgeJobAfter=12month\n# PurgeResvAfter=2month\n# PurgeStepAfter=2month\n# PurgeSuspendAfter=1month\n# PurgeTXNAfter=12month\n# PurgeUsageAfter=12month\n"}` |  Optional raw Slurm configuration files, as a map. The map key represents the config file by name; the map value represents config file contents as a string. Ref: https://slurm.schedmd.com/man_index.html#configuration_files |
| slurm.epilogScripts | map[string]string | `{}` |  The Epilog scripts for compute nodesets, as a map. The map key represents the filename; the map value represents the script contents. WARNING: The script must include a shebang (!) so it can be executed correctly by Slurm. Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_Epilog Ref: https://slurm.schedmd.com/prolog_epilog.html Ref: https://en.wikipedia.org/wiki/Shebang_(Unix) |
| slurm.prologScripts | map[string]string | `{}` |  The Prolog scripts for compute nodesets, as a map. The map key represents the filename; the map value represents the script contents. WARNING: The script must include a shebang (!) so it can be executed correctly by Slurm. Ref: https://slurm.schedmd.com/slurm.conf.html#OPT_Prolog Ref: https://slurm.schedmd.com/prolog_epilog.html Ref: https://en.wikipedia.org/wiki/Shebang_(Unix) |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
