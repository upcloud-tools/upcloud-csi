# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

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

[Unreleased]: https://github.com/upcloud-tools/upcloud-csi/compare/v2.2.0...HEAD
[2.2.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/upcloud-tools/upcloud-csi/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/upcloud-tools/upcloud-csi/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/upcloud-tools/upcloud-csi/releases/tag/1.0.0
