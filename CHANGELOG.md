# Changelog

All notable changes to this project will be documented in this file.

## [2.8.0] - 2026-06-24

### Fixed
- controller: ControllerExpandVolume now returns NodeExpansionRequired: true for unattached volumes, ensuring the filesystem is resized on next mount

### Changed
- ci: narrow test.yaml paths, add types: [go] to pre-commit test hook
- ci: fix trivy upload-sarif category conflict
- `pre-commit-config.yaml`: add `types: [go]` to test hook so `make test` only runs on Go file changes
- docs: added doc comments to upcloud_service.go

### Security
- deps: bump golang.org/x/net v0.51.0 → v0.56.0 (HTTP/2 DoS, Punycode)
- deps: bump github.com/onsi/ginkgo/v2 v2.27.2 → v2.32.0

## [2.7.0] - 2026-06-21

### Added
- ci: cosign keyless image signing in release workflow via GitHub OIDC
- ci: add CHANGELOG.md to build.yaml paths as a more reliable way to trigger a release

## [2.6.1] - 2026-06-21

### Changed
- build: optimize Containerfile — `--no-cache` on apk, remove redundant packages, Go 1.26 dependency consistency
- docs: update README

## [2.6.0] - 2026-06-20

### Added
- metrics: Prometheus metrics server on `--metrics-address` (default `:8090`) with go/process collectors at `/metrics`
- metrics: CSI gRPC operation counters and histograms — `csi_plugin_operations_total`, `csi_plugin_operation_duration_seconds`, `csi_plugin_operations_in_flight`
- metrics: instrumented UpCloud API client wrapper — `upcloud_api_requests_total`, `upcloud_api_request_duration_seconds`

## [2.5.1] - 2026-06-20

### Fixed
- controller: return proper gRPC error on volume resize failures (non-published paths)
- filesystem: wait for storage to be online before calling ResizeStorageFilesystem API

## [2.5.0] - 2026-06-17

### Added
- test: add E2E test for creating VolumeSnapshots and restoring PVCs from snapshots
- test: add snapshot test helpers using dynamic K8s client
- test: run each test in an isolated namespace with labeled cluster-scoped resources
- ci: run e2e tests in parallel via matrix strategy (deploy once, test cases run concurrently)

### Changed
- csi-snapshotter: upgrade to v8.6.0 (registry migration: k8s.gcr.io → registry.k8s.io)
- snapshot-controller: upgrade to v8.6.0
- snapshot-validation-webhook: upgrade to v8.6.0, scoped to volumesnapshotclasses only
- crd: regenerate VolumeSnapshot/Content/Class CRDs from release-8.6 with CEL validation
- rbac: add patch/watch verbs for volumesnapshot resources

### Fixed
- build: use full commit SHA as default IMAGE_TAG to prevent tag mismatch between build and deploy
- test: increase snapshot restore timeout from 3m to 6m (UpCloud clone operation can take ~3.5m)

### Removed
- webhook: drop v1beta1 API version support

### Upgrade notes

Before upgrading to v2.5.0, users with existing VolumeSnapshots need to:

1. **Verify no v1beta1 API usage**: The webhook no longer serves `v1beta1`. Any tooling or automation using `snapshot.storage.k8s.io/v1beta1` must be migrated to `v1` before upgrading. Existing stored objects should already be on `v1` storage version, but verify:

   ```bash
   kubectl get volumesnapshotclasses.snapshot.storage.k8s.io -o json | jq '.items[].apiVersion'
   ```

   All should show `snapshot.storage.k8s.io/v1`.

2. **CRD schema migration**: The new CRDs use CEL validation. Kubernetes will automatically convert stored objects. No manual data migration needed.

3. **Validation webhook change**: Validation for `volumesnapshots` and `volumesnapshotcontents` is now handled by CEL rules in the CRDs — the webhook only covers `volumesnapshotclasses`. If you have custom admission policies relying on the old webhook, review them.

4. **Container registry**: The snapshotter images moved from `k8s.gcr.io` (frozen) to `registry.k8s.io`. Ensure your cluster can pull from `registry.k8s.io/sig-storage/*` (most can, it's the default).

5. **Rollback safety**: If you need to roll back, the old CRDs (release-4.2) may not accept objects created with CEL-populated fields. Keep a backup of the old CRD manifests.

## [2.3.0] - 2026-06-17

### Added
- ci: add Dependabot configuration for automated dependency updates (Go modules, GitHub Actions, Docker)

### Changed
- test: remove btrfs filesystem support from filesystem unit tests (xfs is the only supported fs)
- test: add E2E test case for XFS filesystem resizing with dedicated StorageClass

### Fixed
- ci: remove unsupported `ginkgo.procs` flag from E2E test workflow

## [2.2.0] - 2026-06-17

### Changed
- build: replace host `make build-plugin` with multistage Containerfile build (Go → Alpine)
- build: replace goreleaser with `gh release create` for draft releases
- build: replace Docker with Buildah for container image builds
- build: single-source Go version in Containerfile ARG (remove from Makefile and CI inputs)
- ci: remove Nomad deployment support (`deploy/nomad/`)
- ci: remove build hook from pre-commit config
- ci: change build context from `cmd/upcloud-csi-plugin/` to repo root
- ci: enable caching in golangci-lint workflow
- docs: add fork differentiators and roadmap to README
- docs: update DEVELOPING.md for new build process
- docs: update RELEASING.md for new release process

### Fixed
- build: correct TREE_STATE value (missing `echo` before `echo clean`)
- runtime: add `xfsprogs-extra` package (provides `xfs_growfs` needed for online resize)

### Security
- runtime: switch to minimal Alpine-based multistage image (reduces attack surface)

## [2.1.0] - 2026-06-16

### Added
- service: add support for UpCloud API tokens

### Changed
- dependencies: updated grpc and logrus versions

## [2.0.0] - 2026-06-16

### Added
- online volume expansion: resize PVC while a pod is actively using it without restarting the pod
- e2e test: validate actual filesystem size inside running pod after resize via `df` polling

### Changed
- identity: set volume expansion capability to ONLINE instead of OFFLINE
- node: replace `parted` with `growpart` for non-interactive online partition resize

## [1.3.0]

### Added
- controller: allow any snapshot as volume data source for encrypted volume

## [1.2.0]

### Added
- controller: support for standard storage tier

### Changed
- update upcloud-go-api to v8.6.1
- update Go to 1.22
- update Docker image to Alpine 3.20

## [1.1.0]

### Added

- controller: support for data at rest encryption (using encrypted snapshots as volume source is not supported yet)

## [1.0.1]

### Fixed
- controller: regard volume as unpublished from the node, if node is not found

## [1.0.0]

First stable release

[Unreleased]: https://github.com/upcloud-tools/upcloud-csi/compare/v2.5.1...HEAD
[2.5.1]: https://github.com/upcloud-tools/upcloud-csi/compare/v2.5.0...v2.5.1
[2.5.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v2.3.0...v2.5.0
[2.3.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v2.2.0...v2.3.0
[2.2.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/upcloud-tools/upcloud-csi/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/upcloud-tools/upcloud-csi/releases/tag/1.0.0
