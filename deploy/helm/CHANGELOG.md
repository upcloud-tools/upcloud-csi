# Helm chart changelog

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
