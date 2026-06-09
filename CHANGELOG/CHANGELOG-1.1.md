## v1.1.1

### Added

- Use cosign to sign image artifacts.
- Generate an SBOM that is included in OCI artifacts.
- Add `--namespaces` flag to both the operator and exposed via the Helm Chart.
- Added namespace scoped watching to webhook.

### Fixed

- Fix LoginSet deletion by skipping reconciliation when DeletionTimestamp is
  set.
- Implemented early exit for accounting, controller, and restapi controllers.
- Fixed DaemonSet scale-down where condemned pods were simultaneously
  uncordoned, preventing deletion.
- Update spdystream to resolve CVE-2026-35469.
- Update helm to resolve CVE-2026-35206.
- Fixed Helm logic that caused dcgm.enabled=false to enable dcgm.
- Skip ServiceMonitor deletion when v1.ServiceMonitor is not installed.
- Adds slurm-operator prefix to slurmd container preStop hook.
- Fixed token-controller not passing --token-workers when initializing the
  controller.
- Fixed display of controller and restapi connection info in Helm charting.
- Fix corner case in IsPodFromNodeSet to guard against future callers using the
  function without additional checking.
- Honor service.nodePort on Accountings, Controllers, and RestAPI CRs.
- Adds a Watch on RestAPIs to the SlurmClient Controller.
- Delete SlurmClient when RestAPI not found.
- Implemented deterministic RestAPI selection for SlurmClient reconciliation.
- Exposes webhook failurePolicy and matchPolicy for both validating and mutating
  webhooks.
- GO-2026-5037 GO-2026-5038 GO-2026-5039.
- Prevent modification of ServiceSpec.externalIP for accountings, controllers,
  loginsets, and restapis.
- Fixed cases where a Patch request was issued with an empty patch, causing
  needless interactions.
- Fixed cases where a Status Patch request was issued with an empty patch,
  causing needless interactions.
- Update Slurm conditions on pods such that status patch thrashing does not
  occur.
- Reduce object patch skew by using in memory object to generate patch from.
- Prevent Token from being able to reference a JWT key secret outside of its
  namespace.
- Fixed cases where CRs could reference and use other CRs in other namespaces.
- Fixed cases where the NodeSet partition config string could be used inject
  arbitrary slurm.conf lines, circumventing the intention of the partition
  config field.
- Fixed exploit where pod hostname could be used to write arbitrary slurm.conf
  lines.
- Force disablement of TaintKubeNodes feature for NodeSets.

## v1.1.0

### Fixed

- Fixed case where Token with old jwtHs256Key would return incorrect value from
  CRD helper.
- Controller CRD only requires slurmKey when `external=false`, just like
  Accounting CRD.
- Correctly compute sssd.conf hash to trigger pod updates.
- Corrected the AnnotationAuthJwtKeyHash string to jwt-key-hash.
- Fixes duplicate SlurmNodeState pod conditions.
- Updated google.golang.org/grpc to v1.79.3 to address CVE-2026-33186.
- Fixed case where a NodeSet pod's status was redundantly patched twice.
- Fixed cases where NotFound errors were not handled, causing NodeSet
  reconciliation to prematurely error.
- Fixed shared objects properly applying multiple owners.
- Fixed error caused from a race of multiple NodeSets attempting to create the
  same shared resource.

### Changed

- Hostnames for daemonset scalingMode pods are no longer prefixed with the
  nodeset name or podSpec.Hostname.

### Removed

- Removed webhook validation for `Controller.Spec.ClusterName` with regards to
  valid database table characters.

## v1.1.0-rc1

### Added

- Added flag to expose webhook server address.
- Exposes the Slurm-operator webhook server port via the Helm chart.
- Added command-line flag to expose the namespace in which lease objects are
  created for leader election.
- Exposes control of leader election for slurm-operator and
  slurm-operator-webhook deployments via Helm charting.
- Added PodDisruptionBudget support for slurm-operator and
  slurm-operator-webhook pods.
- JobAcctGatherType will default to `jobacct_gather/linux` when accounting is
  enabled but cgroups is disabled.
- Adds warning to installation guide for Helm 4 bug.
- Added documentation for deploying JupyterLab.
- Implements toggle for NodeSet worker pod PDB creation.
- Adds end-to-end tests for slurm-operator.
- The operator will update Slurm nodes with their topology based on the
  Kubernetes node which the NodeSet pod is allocated to.
- The NodeSet pod's env will now contain POD_TOPOLOGY, which allows the slurmd
  container to start with a topology.
- Adds defaulting behavior for `gres.conf`.
- Added NodeSet `ordinalPadding`, which indicates how many digits to zero pad
  the ordinal with, for better Slurm hostlist compression when replicas are
  greater than 10 (e.g. `slinky-[000-999]`).
- Container images can be expressed as either a string or object.
- Added documentation on using Slurm-operator with SR-IOV
- Added option to enable/disable creation of SlurmKey and jwtHs256Key
- Added support for JWKS through auth/jwt
- Add `app.kubernetes.io/part-of=slurm-operator` to slurm-operator chart
  components.
- Added namespace.yaml as a known Slurm config file in the webhook.
- Add a new field ScalingMode, to control scaling NodeSet Pods like a DaemonSet
  (replica count will be ignored in this mode) or a Statefulset.
- The operator will now recreate NodeSet pods whom either failed to register on
  startup or were unregistered by admin.
- Added Nodeset `persistentVolumeClaimRetentionPolicy` defaults and validation.
- Added configurable deployment strategy to LoginSet.
- Added field JwtKeyRef to accounting, controller, and token CRDs.
- Added upgrade guide.

### Fixed

- Fixes Helm templates for Helm 4.0.
- The `slurmrestd.imagePullPolicy` field properly overrides the global
  `imagePullPolicy`.
- Custom metadata is properly applied to CR and pod template.
- Fixed installing charts with Helm 4, which has stricter syntax requirements
  than Helm 3.
- Fixed key mapping for RSA and ECDSA keys.
- Fixes slurm-operator RBAC configuration to permit leader-election.
- Differentiated operator and webhook LeaderElectionID to fix lease creation
  when using leader election.
- Fixed misconfiguration of `ProctrackType` when `CgroupPlugin=disabled`.
- Configure LoginSet /etc/slurm for SlurmUser access.
- Avoid `storageClassName=null` error, which occurs when using the Slurm chart
  default `values.yaml`, implicitly used by helm.
- prolog/epilog scripts being overwritten instead of merged when multiple
  ConfigMaps are referenced.
- Modified SlurmClient behavior to prevent job termination on Slurm job lookup
  failures.
- Upgrade containerd dependencies to at least v1.7.29 to avoid CVE-2024-25621
  and CVE-2025-64329.
- Upgrade k8s.io/kubernetes to at least v1.34.2 to avoid CVE-2025-13281.
- Upgrade golang.org/x/crypto to at least v0.45.0 to avoid CVE-2025-58181 and
  CVE-2025-47914.
- Update go toolchain to 1.25.5 to fix vulnerabilities.
- Incorrect MemSpecLimit calculation when useResourceLimits=true
- Fixed case where non-alpine images for initconf container would fail to
  execute script.
- Fixed reliance on adduser and addgroup for initconf script, hence any image
  with basic OS utils should work (e.g. Alpine, Ubuntu, Rocky Linux, Alma
  Linux).
- Fixed cases where NodeSets and their Pods were not up to date (replicas,
  status) due to Kubernetes API dropping requests.
- Fixed case where an expired JWT would never be refreshed.
- Fixed idempotency of slurmctld pod volume projection generation.
- Fixed unstable generation of NodeSet and Partition lines in slurm.conf,
  causing unwanted reconfigures.
- Fixed NodeSet and LoginSet templating logic to allow minimal objects to be
  given.
- Fixed case where NodeSet pod scale-in would stall because the controller was
  handling expectations based on stale Slurm client after draining the node.
- Fixed case where a NodeSet pod attempting to be deleted but not drained yet
  would not correctly have its reconcile requeued to try again after it was
  marked DRAIN in Slurm.
- Fixed case where deletion expectations were set incorrectly when above 250
  pods in a single deletion cycle.
- Fixed bug where Helm templates would fail to render multiple loginsets
- Fixed cases where chart values were omitted because they were falsy but a
  valid input.
- Properly default `NodeSet.UpdateStrategy.Type=RollingUpdate` with enum
  validation.
- Fixed case where a rolling update could dereference a nil pointer.
- Fixed template error when ref is nil.

### Changed

- Custom Resource (CR) metadata to its derived objects (e.g. pods, service).
- Moved LoginSet sssd.conf configuration to a top level object for cluster-wide
  configuration.
- Token requests are requeued with a duration based on their refresh time, which
  reduces unnecessary reconcile cycles.
- Changed the Token's default JWT expiration to 1 hour to improve security.
- Do not render unset NodeSet fields.
- Allow NodeSet `updateStrategy.type=""` to better allow minimal NodeSet objects
  to be applied.
- Converted Slurm and JWT key references to objects containing secretRefs.
- Renamed jwtHs256Key to jwtKey in Slurm Helm chart.
- Move the setting of slurmd pod resource limit environment variables into
  slurm-operator from Helm chart.
- Default slurm.key and jwt_hs256.key are no longer kept upon chart
  uninstallation by default.
- Change default of `NodeSet.RollingUpdate.MaxUnavailable` to `25%`.
- NodeSet `partition.enabled` now defaults to `false`.
- Switched Helm Charting to use generic JwtKey fields.
