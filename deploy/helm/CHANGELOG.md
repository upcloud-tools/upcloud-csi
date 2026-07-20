# Helm chart changelog

## [1.14.1] - 2026-07-20

### Changed
- Include LICENSE and README in chart
- Update .helmignore to exclude tests

## [1.14.0] - 2026-07-20

### Changed
- Bump app version to `v3.2.0`

## [1.13.1] - 2026-07-18

### Changed
- Set FileStorage reclaimPolicy to Retain by default

## [1.13.0] - 2026-07-18

### Changed
- Bump app version to `v3.1.0` (NFS File Storage support - dynamic provisioning)

## [1.12.0] - 2026-07-17

### Changed
- Bump app version to `v3.0.0` (NFS File Storage support - static provisioning [BETA])

## [1.11.2] - 2026-07-15

### Changed
- Bump app version to `v2.8.5`

## [1.11.1] - 2026-06-30

### Changed
- Bump app version to `v2.8.4` (Go 1.26.4, Alpine 3.23.5)

## [1.11.0] - 2026-06-30

### Added
- `clusterZone` value for both controller and node. Setting it avoids an API call on the controller startup, since it's needed for the DaemonSet already.

### Changed
- Node DaemonSet runs `--mode=node` now instead of `--mode=monolith`. UpCloud API credentials are no longer deployed to every cluster node, reducing credential blast radius
- Controller StatefulSet and Node DaemonSet use `clusterZone` for the `--zone` flag

## [1.10.0] - 2026-06-28

### Added
- Chart packages are signed too now

## [1.9.3] - 2026-06-26

### Changed
- Update app version to `v2.8.3`

## [1.9.2] - 2026-06-26

### Changed
- Release now created as draft — allows time to fix app releases before chart becomes available
- Update app version to `v2.8.2`

## [1.9.1] - 2026-06-24

### Changed
- Update app version to `v2.8.1`

## [1.9.0] - 2026-06-24

### Changed
- Update app version to `v2.8.0`

## [1.8.0] - 2026-06-23

### Added
- `snapshotValidationWebhook.certManager` — auto-provision TLS via cert-manager with self-signed Issuer or existing issuer
- E2E test for snapshot validation webhook — verifies admission control accepts valid VolumeSnapshotClasses and rejects invalid ones
- `snapshotValidationWebhook.podDisruptionBudget` with PDB template for HA deployments

### Changed
- `snapshot-webhook.yaml`: switched to port 8443 (unprivileged), tcpSocket probes, removed `--http-endpoint` flag
- `NOTES.txt` now shows a warning when webhook TLS secret is missing and suggests cert-manager
- Webhook image updated to `v8.1.1` (latest available snapshot-validation-webhook tag)
- `snapshot-controller-deployment.yaml`: fixed duplicate `strategy` key in template source

## [1.7.0] - 2026-06-22

### Added
- `UPCLOUD_TOKEN` env var in controller and node pods — reads the token key from the credentials secret for Bearer token auth
- `controller.zone` value — passes `--zone` to the driver for explicit zone configuration
- `credentials.token` value — used when `createSecret=true` to create a secret with the API token

### Changed
- `UPCLOUD_USERNAME` and `UPCLOUD_PASSWORD` env vars use `optional: true`
- `credentials.tokenKey` value configures which secret key holds the UpCloud API token
- `NOTES.txt` now suggests token-based secret as the preferred option

### Removed
- `credentials.username` and `credentials.password` removed. When `createSecret=true`, only a token secret creation is supported. The chart will still use username + password from an existing secret by default.

## [1.6.0] - 2026-06-21

### Changed
- App version bumped to `v2.7.0`

### Added
- `networkPolicy` block with opt-in NetworkPolicy resources for controller, node, snapshot-controller, and webhook
- E2E test for NetworkPolicy ingress enforcement — verifies blocked ports are unreachable
- Enhanced `NOTES.txt` with runtime credential secret check, Prometheus metrics notice, and NetworkPolicy notice

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
