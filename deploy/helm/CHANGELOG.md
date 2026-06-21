# Helm chart changelog

## [1.6.0] - 2026-06-21

### Added
- `networkPolicy` block with opt-in NetworkPolicy resources for controller, node, snapshot-controller, and webhook
- E2E test for NetworkPolicy ingress enforcement — verifies blocked ports are unreachable

## [1.5.1] - 2026-06-20

### Changed
- App version bumped to `v2.6.1`
- Containerfile optimized — smaller runtime image

### Added
- `networkPolicy` block with opt-in NetworkPolicy resources for controller, node, snapshot-controller, and webhook

### Fixed
- Remove `runAsNonRoot` from controller, snapshot-controller, and webhook `podSecurityContext` defaults — CSI sidecar images run as root and cannot be launched with this constraint

## [1.5.0] - 2026-06-20

### Changed
- App version bumped to `v2.6.0`

### Added
- `metrics` block with configurable ServiceMonitor and PrometheusRule support — controller sidecars now expose `--http-endpoint` on standard ports (8080-8083) and a ClusterIP metrics Service is created by default
- Driver metrics port (`csi-metrics:8090`) on controller and node, wired into metrics Service and ServiceMonitor

## [1.4.0] - 2026-06-20

### Added
- `extraObjects` support — deploy arbitrary Kubernetes resources with Go template support
- `imagePullSecrets` per component for private registry authentication (controller, node, snapshotController, snapshotValidationWebhook)
- Configurable `updateStrategy`, `terminationGracePeriodSeconds`, `lifecycle`, `topologySpreadConstraints`, `runtimeClassName`, `dnsPolicy`/`dnsConfig`, `hostAliases`, `initContainers`, `additionalVolumes`/`additionalVolumeMounts`, `minReadySeconds`, and `revisionHistoryLimit` per component
- `securityContext` and `podSecurityContext` per component with secure defaults — controller/snapshot/webhook drop all capabilities with read-only rootfs and runAsNonRoot; node keeps privileged defaults with pod-level seccomp

## [1.3.0] - 2026-06-20

### Added
- `commonLabels` applied to all resource metadata via `_helpers.tpl`
- Per-component `podLabels` and `podAnnotations` for controller, node, snapshot-controller, and webhook
- `serviceAccount.annotations` for controller and node service accounts
- Helm unit tests for all templates with 60 assertions across 9 test suites
- values.schema.json for Helm values validation

### Changed
- All `toYaml` renders of user-supplied values now wrapped with `tpl()` to support template expressions in values

## [1.2.1] - 2026-06-20

### Added
- Liveness and readiness probes for csi-upcloud-plugin driver, snapshot-controller, and snapshot-validation-webhook

## [1.2.0] - 2026-06-20

### Changed
- StorageClasses are now disabled by default to avoid conflicts during installation (`storageClasses.enabled: false`)
- Consolidated install/upgrade docs to use `helm upgrade --install`

### Added
- VolumeSnapshot CRD conflict resolution instructions to README
- `--skip-crds` install option for clusters with pre-existing CRDs
- Upgrade instructions to README
- Warning block before destructive snapshot deletion commands

## [1.1.0] - 2026-06-19

### Added
- PodDisruptionBudget for controller StatefulSet and snapshot-controller Deployment (opt-in)

### Fixed
- NOTES.txt credential key references now use configurable key names instead of hardcoded values

## [1.0.0] - 2026-06-18

Initial release of the UpCloud CSI Helm chart.

### Features
- Controller deployment with external-provisioner, external-attacher, external-resizer, and external-snapshotter sidecars
- Node DaemonSet with node-driver-registrar sidecar
- Snapshot controller and validation webhook deployments
- Configurable StorageClasses (maxiops, hdd, standard) with default class support
- VolumeSnapshotClass with configurable deletion policy
- Per-component log levels, resource requests/limits, and node selectors
- Credentials secret management (auto-create or reference existing)
- RFC 1123 compliant container names
