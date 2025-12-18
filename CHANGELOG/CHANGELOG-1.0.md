## v1.0.1

### Added

- JobAcctGatherType will default to `jobacct_gather/linux` when accounting is
  enabled but cgroups is disabled.

### Fixed

- Fixed installing charts with Helm 4, which has stricter syntax requirements
  than Helm 3.
- Fixed misconfiguration of `ProctrackType` when `CgroupPlugin=disabled`.
- Avoid `storageClassName=null` error, which occurs when using the Slurm chart
  default `values.yaml`, implicitly used by helm.
- prolog/epilog scripts being overwritten instead of merged when multiple
  ConfigMaps are referenced.
- Upgrade k8s.io/kubernetes to at least v1.34.2 to avoid CVE-2025-13281.
- Upgrade golang.org/x/crypto to at least v0.45.0 to avoid CVE-2025-58181 and
  CVE-2025-47914.
- Configure LoginSet /etc/slurm for SlurmUser access.

## v1.0.0

### Fixed

- Fixed Helm templates for Helm 4.0

## v1.0.0-rc1

### Added

- The Slurm Helm Chart can now be configured with `PrologSlurmctld` and
  `EpilogSlurmctld`.
- Add arm64 support and multiarch manifest.
- Added NodePort to `v1alpha1.ServiceSpec`.
- Added pod hostname resolution of NodeSet pods.
- Adds hostname label to pods for Slurm node mapping.
- Synchronize Kubernetes node [un]cordon state to NodeSet pods and their Slurm
  nodes. When Kubernetes nodes are cordoned, NodeSet pods running on those nodes
  are also cordoned and their Slurm nodes drained. Those NodeSet pods remain
  cordoned until the Kubernetes node becomes uncordoned.
- Implements graceful nodeset pod disruption handling
- Added metrics-server-bind-address command-line option for the slurm-operator
  controller.
- Added liveness probe to slurmrestd container, which will restart its pod if it
  becomes unresponsive long enough.
- Custom Slurm node drain message for kubectl cordon.
- Adds dynamic node tainting.
- Can now support hybrid clusters, where one or more Slurm components exist
  externally to Kubernetes but be joined to the same Slurm cluster.

### Fixed

- Fixes parsing of `ServiceSpec` via `ServiceSpecWrapper`.
- Correctly use global imagePullPolicy as the default value for all containers.
- Determine cluster domain instead of assuming the default (`cluster.local`).
- Update kubeVersion parsing to handle provider suffixes (e.g., GKE
  `x.y.z-gke.a`).
- Fixed odd number of arguments logger error when updating pod conditions.
- Avoid needless NotFound errors when patching pod conditions.
- Fixed regression where nodeset `partition.enabled` was not being respected.
- Initial NodeSet no longer accidentally owns the worker service.
- Fixed issue where changes to slurmd and/or logfile subobjects where not
  causing a rolling update.
- Fixed notation used to refer to LoginSets in installation docs.
- Fixed documentation for uninstalling slurm-operator-crds.
- When checking if a Slurm node is fully drained, the logic now follows closely
  to how Slurm represents the drained state. There were certain edge cases that
  could alleged the node was not drained when it actually was.
- Check if Slurm node is [un]drain before requesting the opposite. This avoids a
  race condition where an admin or script has applied [un]drain to the Slurm
  node but the operator is not aware of it.
- When Slurm nodes are put into drain state, the provided reason should not be
  thrashed by subsequent drain requests.
- Fixed installation instruction for cert-manager chart.
- Fixes bug wereby slurm-controller hostname was set incorrectly.
- Fixes per-nodeset partition creation.
- Fixed chart installation failure where NOTES.txt failed to fetch value from
  nested object where the parent was null.
- Fixed imagePullPolicy in slurm-operator Helm chart.
- Fixes edge case where Slurm node state is not reset when a worker pod migrates
  kube nodes.
- Reduce checksum collision during file change detection by using SHA256 instead
  of MD5.
- When `CgroupPlugin=disabled`, do not configure `PrologFlags=Contain` and other
  parameters that depend on it.
- Added liveness probe to slurmd container to restart the pod if slurmd crashes
  after starting.
- Prevent Slurm node undrain when node is down or notresponding.
- Fixed reason prefixing behavior in MakeNodeUndrain.
- Default webhook timeout is now consistent across all endpoints, respecing the
  user input, otherwise using the Kubernetes default.
- Fixed case where multiple env variables in LoginSet would cause the operator
  to keep updating the LoginSet Deployment causing the underlying ReplicaSet to
  endlessly thrash.
- Fixed case where NodeSets being added or removed from the Slurm cluster was
  not triggering a reconfigure.

### Changed

- Organized documentation into sub-directories.
- Updates the paths used to refer to the user's home directory in installation
  instructions.
- Slurm node [un]drain activity now includes more context.
- Made the NodeSet updateStrategy configurable in the Slurm helm chart. The
  default minUnavailable was changed to 25%.
- Shortened naming schema for health and metrics addresses.
- Exposed addresses for health and metrics of the slurm-operator controller pod
  via the Helm chart.
- slurmctld - The reconfigure container is now a sidecar instead of main
  container.
- Reduced interval of the reconfigure check. After the kubelet updates mounted
  files in the pod, a reconfigure will be issued more quickly.
- All supplemental containers are now `corev1.Container`, allowing full
  configuration.
- Chart metadata is no longer applied to the pod template.
- Updated NodeSet pod preStop to better indicate why the Slurm node was set to
  DOWN before deletion.
- Service metadata is now configurable separately from the pod template
  metadata.
- Webhooks avoid kube-system namespace.
- Replaced slurm-exporter with a serviceMonitor that scrapes slurmctld directly.
- Move to Slurm v44 API (from v43).

### Removed

- Removed defaulting webhooks.
- Removed v1alpha1 CRDs to cleanly delineate v1 from v0 releases. Going forward,
  old versions of CRDs in v1 releases will linger in a deprecated state and be
  removed in future releases as needed.
