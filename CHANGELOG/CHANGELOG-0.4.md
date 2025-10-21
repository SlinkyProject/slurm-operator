## v0.4.1

### Added

- Added NodePort to v1alpha1.ServiceSpec

### Fixed

- Fixes links to installation.md
- Fixed conversion of GHFM admonitions to RST
- Fixed image rendering in docs/index.rst
- Update kubeVersion parsing to handle provider suffixes (e.g., GKE
  x.y.z-gke.a).
- Fixed odd number of arguments logger error when updating pod conditions.
- Avoid needless NotFound errors when patching pod conditions.
- Fixed regression where nodeset `partition.enabled` was not being respected.
- Fixed issue where changes to slurmd and/or logfile subobjects where not
  causing a rolling update.
- Fixes parsing of ServiceSpec via ServiceSpecWrapper
- Fixes bug wereby slurm-controller hostname was set incorrectly
- Fixes per-nodeset partition creation
- Fixed chart installation failure where NOTES.txt failed to fetch value from
  nested object where the parent was null.
- Fixed installation instruction for cert-manager chart
- Fixed notation used to refer to LoginSets in installation docs
- When checking if a Slurm node is fully drained, the logic now follows closely
  to how Slurm represents the drained state. There were certain edge cases that
  could alleged the node was not drained when it actually was.
- Fixed documentation for uninstalling slurm-operator-crds

## v0.4.0

### Added

- Added NodeSet level authcred configuration.
- Added topology.yaml to config files
- Added Accounting, Controller, Restapi, and LoginSet CRDs.
- Added Slurm ClusterName override, otherwise derived from Controller CR
  metadata.
- Added disaggregated configuration for each sidecar, no longer overloading the
  authcred configuration.
- Added liveness probe for Controller pod
- Added slurm-operator-crds helm chart to manage CRDs.
- Added slurm-operator helm chart options for CRDs.
- Added `initContainers` to Slurm helm chart.
- Added the Token CRD to define a secret containing a JWT, generated from an
  HS256 Key.
- Added `NVIDIA/dcgm-exporter` integration to Slurm helm chart.
- Added conditions to slurmd pods to reflect Slurm node state
- Added helm chart `kubeVersion` requirements.

### Fixed

- Fixed token job `ttlSecondsAfterFinished` being too low for helm
  `--wait-for-jobs`.
- Fixed nodeset pod's sackd image tag default value.
- Fixed webhook timeout being read from unintended values path.
- Fixed operator replicas being read form unintended values path.
- Fixed race condition where a stale NodeSet pod status leads to the Slurm node
  being terminated instead of drained.
- Fixed slurm-exporter chart dependency not being in lock-step with release.
- Fixed passing additional arguments to the slurmrestd container.

### Changed

- Changed default storageClassName to empty.
- Changed to Slurm v43 API.
- Changed how a Slurm clusters are expressed via CRDs.
- Changed lifetime of JWT tokens created by operator from infinite to 15
  minutes.
- Changed how Slurm config files and secrets are set up in the pod, mount
  volumes with `securityContext.fsGroup` and remove initconf sidecar.
- Changed logfile sidecar image to alpine.
- Changed reconfigure sidecar image to slurmctld.
- Changed Slurm chart to use Token CR for slurm-exporter JWT.
- Changed ConfigFiles, PrologScripts, and EpilogScripts to list of object
  references.
- Changed `nodeset.spec.template.extraConf` => `nodeset.spec.extraConf`.
- Changed Token CR field JwtHs256KeyRef to include an optional namespace.

### Removed

- Removed the Cluster CRD.
- Removed `bitnami/mariadb` dependency from Slurm helm chart.
