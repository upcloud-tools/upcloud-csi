# Changelog

All notable changes to this project will be documented in this file.
See updating [Changelog example here](https://keepachangelog.com/en/1.0.0/)

## [Unreleased]

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

[Unreleased]: https://github.com/UpCloudLtd/upcloud-csi/compare/v2.1.0...HEAD
[2.1.0]: https://github.com/UpCloudLtd/upcloud-csi/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/UpCloudLtd/upcloud-csi/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/UpCloudLtd/upcloud-csi/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/UpCloudLtd/upcloud-csi/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/UpCloudLtd/upcloud-csi/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/UpCloudLtd/upcloud-csi/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/UpCloudLtd/upcloud-csi/releases/tag/1.0.0
