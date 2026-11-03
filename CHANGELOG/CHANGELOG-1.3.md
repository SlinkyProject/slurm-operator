## v1.3.0-rc1

### Added

- Added more controller events.
- Add `--namespaces` flag to both the operator and exposed via the Helm Chart.
- Added Slurm parameter option appending to avoid user extraConf from clobbering
  required parameter values.
- Added extraObjects to charts to express extra Kubernetes manifests to be
  managed with the chart.
- Added NodeSet webhook checks block unwanted updates where controllerRef
  changes or name is empty, volumeClaimTemplates changes, sssdConfRef name is
  empty, and maxUnavailable value is < 0.
- Added LoginSet webhook checks to block unwanted updates where controllerRef is
  empty or changes, and sssdConfRef name is empty.
- Added authentication hashes to Controller pod, which will trigger a recreate
  if they ever change.
- Added logic to allow override of LoginSet SSH port.
- Slurm node drain reason can now be set from configured Kubernetes node
  conditions, enabling health check tooling to propagate context into Slurm.
- Added option to allow NodeSet pods in StatefulSet scaling mode to be pinned to
  the Kubernetes node where they were originally running.
- Added nodeset.slinky.slurm.net/hostname-override node annotation to override
  DaemonSet pod hostnames.
- Use cosign to sign image artifacts.
- Generate an SBOM that is included in OCI artifacts.
- Controller pod can now be recreated or inplace reconfigured when Slurm config
  changes.
- Added NodeSet `pruneSlurmNodeRecords` option to indicate how Slurm node
  records are pruned, if at all.
- Added NodeSet `pruneSlurmNodeRecords=NodeNotFound` option where the operator
  will prune Slurm node records of pods where the backing Kubernetes node no
  longer exists, but only for DaemonSet mode.
- TopologySpreadConstraints value hook for the slurm-operator and
  slurm-operator-webhook Deployments.
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
- Added optional PodDisruptionBudget for RestAPI.
- Expose webhook.validating.namespaceSelector and exclude kube-node-lease from
  the default webhook scope.
- Added tutorial for GKE TPU deployment.
- Added support for Slurm node feature synchronization from a known Kubernetes
  node annotation.
- Added documentation for RestAPI job submission.
- Expose validating and mutating webhook matchConditions.
- Added A3 Ultra Machines to vendored Helm charting.
- Annotation describing Slinky minimum supported Slurm version.
- Use Slurm virtual IP (SlurmctldAddr) to reduce cases where backup controller
  fail over experiences TCP Timeouts because the primary cannot respond.
- Support for external TLS certificates.
- Add optional Prometheus and Grafana monitoring for development.
- LoginSet pods are now labeled with loginset.slinky.slurm.net/pod-name,
  matching NodeSet's convention.
- Extended Helm NodeSet tests to cover DRA-style resource claims and assertions.
- Add toggle for NodeSet topology sync.
- Added Auto option to NodeSet pruneSlurmNodeRecords, which deletes Slurm node
  records for nodes stuck in INVALID_REG state.
- Added KWOK support to `hack/kind.sh` for scale testing the operator.
- Add SYS_RESOURCE to login pods so they can adjust the oom score of supervisord
  services.
- Added reference deployments for ArgoCD.
- Expose environment variables for slurmdbd and slurmctld.
- Added `pruneSlurmNodeRecords=Always` option to aggressively manage Slurm node
  records.
- Added new `ScaleStrategy` to Slurm chart.
- Added ability for condemn candidates to switch to pods with an earlier
  deadline.

### Fixed

- Fixed case where Token with old jwtHs256Key would return incorrect value from
  CRD helper.
- Controller CRD only requires slurmKey when `external=false`, just like
  Accounting CRD.
- Correctly compute sssd.conf hash to trigger pod updates.
- Corrected the AnnotationAuthJwtKeyHash string to jwt-key-hash.
- Fixes duplicate SlurmNodeState pod conditions.
- Fixed case where a NodeSet pod's status was redundantly patched twice.
- Fixed cases where NotFound errors were not handled, causing NodeSet
  reconciliation to prematurely error.
- Updated google.golang.org/grpc to v1.79.3 to address CVE-2026-33186.
- Fixed nil pointer dereference on probe handler merge during container build.
- Fixed shared objects properly applying multiple owners.
- Fixed error caused from a race of multiple NodeSets attempting to create the
  same shared resource.
- Fixed operator panic when toggling accounting on or off.
- Return error instead of panicking when an error is encountered while
  generating cryptographic keys.
- Set action field in Eventf to prevent rejection.
- Fix LoginSet deletion by skipping reconciliation when DeletionTimestamp is
  set.
- Implemented early exit for accounting, controller, and restapi controllers.
- Fixed DaemonSet scale-down where condemned pods were simultaneously
  uncordoned, preventing deletion.
- Update spdystream to resolve CVE-2026-35469.
- Update helm to resolve CVE-2026-35206.
- Fixed Helm logic that caused dcgm.enabled=false to enable dcgm.
- GO-2026-4864 GO-2026-4865 GO-2026-4866 GO-2026-4869 GO-2026-4870 GO-2026-4946
  GO-2026-4947.
- Skip ServiceMonitor deletion when v1.ServiceMonitor is not installed.
- Fix corner case in IsPodFromNodeSet to guard against future callers using the
  function without additional checking.
- Fixed token-controller not passing --token-workers when initializing the
  controller.
- Adds slurm-operator prefix to slurmd container preStop hook.
- Honor service.nodePort on Accountings, Controllers, and RestAPI CRs.
- Fixed display of controller and restapi connection info in Helm charting.
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
- Do not emit error when service monitor GVK is not registered.
- Fixed cases where status updates would cause continuous reconciles under
  certain conditions.
- Fixed NodeSet rolling and scheduled updates that could delete healthy old pods
  instead of unhealthy ones, by partitioning pods on health rather than creation
  timestamp.
- Fixed NodeSet pod deletion ordering so equally cordoned pods are ordered by
  the remaining tiebreakers instead of arbitrarily.
- Fixed NodeSet PVC creation events reporting a misleading not found failure for
  successfully created claims.
- Fixed NodeSet rolling updates condemning old pods past maxUnavailable while
  replacement pods were still unavailable, which could drain a NodeSet to zero
  available workers on a slow or broken image pull.
- Aggregate syncNodeSetPodStatus errors in syncStatus.
- Adds sanitization when setting Slurm node drain reason from Node annotation.
- Grant secrets/finalizers RBAC to token-controller.
- Fixed a panic in the NodeSet, RestApi, and LoginSet event handlers when
  listing referenced CRs failed.
- Fix security vulnerabilities.
- GO-2026-4970 GO-2026-5856.
- Fixed slurmrestd not being restricted to the slurmctld openapi plugin when the
  Controller has no accountingRef.
- Fixed the metrics-secure flag being accepted but never applied; setting it now
  serves metrics via HTTPS with authentication and authorization. Also fixed
  metricsPort 0 not disabling the operator metrics endpoint.
- GO-2026-5942 GO-2026-5970.
- Fixed log level of reconciliation errors to ensure stack trace generation.
- GO-2026-5158.
- GO-2026-5841 GO-2026-6061.
- Align Helm AppVersion and Version with Slinky Version.
- Fixed end-to-end tests for new nodeset defaulting behavior.
- Fixed scheme registration to be compatible with controller-runtime deprecation
  of scheme builder.
- Replace /readyz probe with StartedChecker instead of a ping.
- Reuse Helm-native serving cert on upgrade.
- Make pods/binding webhook dry-run aware.
- Add a 5m timeout to all slurm rest calls.
- Fixed cases where an incorrect requeueAfter duration was applied to the
  reconcile request, causing the Token to stop being reconciled and its JWT
  could expire without refresh.
- Rolling update no longer flaps Slurm drain when replacement pod is
  unschedulable.
- Fixed cases where the webhooks would intercept pods in the kube-public
  namespace.
- GO-2026-5026 GO-2026-5942 GO-2026-5972 GO-2026-6088 GO-2026-6089 GO-2026-6090
  GO-2026-6091 GO-2026-6179 GO-2026-6180 GO-2026-6218.
- Cross-namespace owner reference error on NodeSet Service/PDB sync.
- Add a 10 second idle connection timeout to all Slurm RESTAPI calls.
- SyncScheduledUpdate now passes full pod list to splitUpdatePods.
- Scale-in no longer flip-flops candidate pods between reconciles.
- NodeSet status no longer goes stale while terminating.
- Updated DaemonSet node record pruning to match on labelselectors.
- Scale-down no longer uncordons a drain-backlog pod.
- Correct durationStore cleanup key on panic recovery.
- GO-2026-6094.
- NodeSet current-revision replica status was never tracked.
- Webhook ClusterRoleBinding now binds to the webhook ClusterRole.
- Fixed cases where `inplaceReconfigure=false` did not properly restart the
  controller pod when additional config files changed.
- GO-2026-6354 GO-2026-6355.
- NodeSets no longer error-loop when their temporary MAINT reservation doesn't
  exist yet, working around a Slurm quirk where that GET reports HTTP 422
  instead of 404.
- A non-exec slurmd preStop handler rendered an invalid container.
- LoginSet SSH host keys are reused instead of regenerated on every reconcile.
- Add CAP_SYS_RESOURCE to slurmd container.
- Fixed case where a NodeSet in StatefulSet mode would count an out of band pod
  termination against the total when deciding to scale, causing spurious pod
  recreations.
- GO-2026-6348 GO-2026-6441 GO-2026-6443.
- Match Slurm host-list parsing for job nodes and reservations.
- NodeSets recover quickly when Slurm becomes reachable after startup.
- NodeSet extraConf and partition config are now quoted, so values containing
  YAML-significant characters no longer break the render or get silently
  truncated.
- Explicitly-set false/0 values in CR templates (e.g. allowPrivilegeEscalation:
  false) are no longer dropped from the built workload.
- Give DaemonSet-mode pods the standard daemon tolerations.
- Skip drain/undrain requests for Slurm nodes in INVALID_REG state.
- GO-2026-6505 GO-2026-6505 GO-2026-6505.
- Render GHFM alerts correctly in generated docs.
- GO-2026-6599 GO-2026-6600 GO-2026-6603 GO-2026-6603 GO-2026-6604 GO-2026-6605
  GO-2026-6607 GO-2026-6608 GO-2026-6609 GO-2026-6610 GO-2026-6610 GO-2026-6611
  GO-2026-6611 GO-2026-6612 GO-2026-6612 GO-2026-6613 GO-2026-6617 GO-2026-6617.
- Fixed cases where `doPodScale` would immediately error if `syncPodUncordon`
  errored instead of collecting the errors and attempting the rest of the pod
  operations.
- Fixed operator mutation request throughput via parallel batching instead of
  ramping parallel batching.
- Use ConcurrentStartBatch instead of raw loop iterators for Kubernetes API
  transactions.
- Operator no longer crashes when a NodeSet pod has no annotations.
- Operator no longer runs out of memory on a NodeSet with very large replicas.
- Drop the Accounting self-watch that reconciled the wrong objects.
- Skip empty object patches that wasted CPU and memory.
- Slurm nodes stay unschedulable while their NodeSet pod terminates.
- Slurmd keeps its spool directory across container restarts.
- Fixed cases where a ErrNoClient was not tolerated.
- GO-2026-6809.
- Stop Kind setup when cluster discovery fails.
- Fixed case where a helm chart upgrade would fail due to `startTime=null`
  despite not being rendered.
- Avoid missing or duplicate NodeCordon events.
- Restapi PDB no longer sets both maxUnavailable and minAvailable.
- GO-2026-6860 GO-2026-6866.
- Allow Slurm node status sync for unhealthy pods.

### Changed

- Hostnames for daemonset scalingMode pods are no longer prefixed with the
  nodeset name or podSpec.Hostname.
- Changed LogTimeFormat to thread_id.
- Initial cluster reconfigure is triggered by `reconfig_on_restart` instead of
  reconfigure sidecar.
- The Slurm node will have its drain reason continually updated, instead of set
  once even if the reason would have changed.
- Marked NodeSet `taintKubeNodes` option as deprecated, and will be removed in
  the future.
- The default NodeSet preStop does not attempt to delete the Slurm node record.
- Changed inplaceReconfigure default to false.
- Raised the default RSA bit length for the Login pod SSH host key to 4096 bits.
- Improved PDB handling for operator and webhook.
- Pruned default `slurm.conf` of all non-required options.
- Update helm, kubernetes, and controller-runtime to be compatible with a newer
  slurm-client.
- Update slurm client token rotation.
- Disabled pods/binding webhook by default.
- Controllers now use a tuned rate limiter instead of the 5ms default.
- Enable Slurm Operator webhook metrics by default.
- Increase e2e nodeset timeout.
- Wait for generated workloads in e2e tests.
- Changed max NodeSet pod creations tp 256, from 250.
- Changed Slurm client HTTP client timeout to 2 minute, from 5 minutes.
- Stop caching managed fields to reduce memory use.
- Updated documentation to keep in sync with repo.
- Webhook only caches metadata of pods, and do longer caches Pods (metadata
  only), Nodes, ConfigMaps, and PVCs.
- Look up controller and accounting references with field indexes.
- Pod who have their metadata or status patched are now only cache (e.g. slurmd,
  slurmctld, and login pods).
- Stop caching PVCs, PodDisruptionBudgets, and ServiceMonitors.
- Improve reconcile throughput by avoiding forced synchronous Slurm node cache
  refresh per reconcile.
- Deprecate `pruneSlurmNodeRecords=NodeNotFound` in favor of
  `pruneSlurmNodeRecords=Auto`.
- Added ScaleStrategy and deprecated fields in favor of ScaleStrategy nested
  fields.
- Changed LoginSet and NodeSet ExtraSshdConfig to be deprecated and optional.
- Changed LoginSet and NodeSet SssdConfigRef to be deprecated and optional.
- Can now safely cordon and delete NodeSet pods in a single reconcile.
- Changed conflated `pod-cordon` by splitting intents with the addition of
  `pod-condemned` and `pod-cordon-requested` annotations.
- Tolerate Pod not found errors when attempting to patch all on all NodeSet
  pods.

### Removed

- Removed irrelevant `CommunicationParameters=block_null_hash`, and redundant
  default `SelectTypeParameters=CR_Core_Memory`.
- Removed webhook validation for `Controller.Spec.ClusterName` with regards to
  valid database table characters.
- Removed default configured NodeSet and LoginSet.
- Removed the default partition "all".
- Omit non-required `gres.conf`.
- Omit deprecated extraSshdConfig and sssd configurations.

### Miscellaneous

- Updated images to default to 26.05-ubuntu26.04.
