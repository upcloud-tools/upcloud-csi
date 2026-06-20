# Helm chart changelog

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
