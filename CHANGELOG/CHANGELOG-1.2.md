## v1.2.0

### Added

- Added optional PodDisruptionBudget for RestAPI.

## v1.2.0-rc1

### Added

- Adds securityContext and podSecurityContext to the Helm chart for the
  slurm-operator and slurm-operator-webhook deployments.
- Added documentation on IMEX configuration.
- Helm chart fields `operator.nodeSelector` and `webhook.nodeSelector` for
  `slurm-operator`, mirroring the existing `affinity` and `tolerations` fields.
- Added namespace scoped watching to webhook.
- Added new variable to set global NodeSet and LoginSet object defaults, which
  cab be overwritten in the nodesets and loginsets map.
- Added `image.digest` field.
- Enabled live Go profiling using pprof for Slurm-operator.
- Added NodeSet oversubscribeNode option to allow more than one NodeSet Pod to a
  Kubernetes Node (useful for testing).
- Adds ScheduledUpdate UpdateStrategy for NodeSets.
- Added basic support for Google GKE A3 Mega instance type in the Slurm chart.

### Fixed

- GO-2026-4864 GO-2026-4865 GO-2026-4866 GO-2026-4869 GO-2026-4870 GO-2026-4946
  GO-2026-4947.
- Adds slurm-operator prefix to slurmd container preStop hook.
- Honor service.nodePort on Accountings, Controllers, and RestAPI CRs.
- Adds a Watch on RestAPIs to the SlurmClient Controller.
- Delete SlurmClient when RestAPI not found.
- Implemented deterministic RestAPI selection for SlurmClient reconciliation.
- Exposes webhook failurePolicy and matchPolicy for both validating and mutating
  webhooks.
- Prevent modification of ServiceSpec.externalIP for accountings, controllers,
  loginsets, and restapis.
- Removed TaintKubeNodes feature from NodeSets.
- GO-2026-5037 GO-2026-5038 GO-2026-5039.
- Prevent Token from being able to reference a JWT key secret outside of its
  namespace.
- Fixed cases where a Patch request was issued with an empty patch, causing
  needless interactions.
- Fixed cases where a Status Patch request was issued with an empty patch,
  causing needless interactions.
- Update Slurm conditions on pods such that status patch thrashing does not
  occur.
- Reduce object patch skew by using in memory object to generate patch from.
- Improved chart traversal over nested nullable objects.
- Fixed cases where CRs could reference and use other CRs in other namespaces.
- Fixed cases where the NodeSet partition config string could be used inject
  arbitrary slurm.conf lines, circumventing the intention of the partition
  config field.
- Fixed exploit where pod hostname could be used to write arbitrary slurm.conf
  lines.
- Fixed logfile containers blocking quick terminating of the pod.
- Fixed cases where slurmctld would get blocked by fifo_open on an unlinked
  inode from logfile sidecar.
- Fixed case where the node reason would be overwritten on subsequent reconcile
  despite already being drained for a different reason.
- Updated default Helm SSSD configuration for newer SSSD releases.

### Changed

- Raised the default RSA bit length for the Login pod SSH host key to 4096 bits.
- Improved PDB handling for operator and webhook.
- Pruned default `slurm.conf` of all non-required options.

### Removed

- Removed default configured NodeSet and LoginSet.
- Removed the default partition "all".
- Omit non-required `gres.conf`.

### Miscellaneous

- Updated images to default to 26.05-ubuntu26.04.
