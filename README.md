# UpCloud CSI Driver
![GitHub Actions status](https://github.com/upcloud-tools/upcloud-csi/actions/workflows/deploy.yaml/badge.svg)

## Overview

UpCloud [CSI](https://github.com/container-storage-interface/spec) Driver provides a basis for using the UpCloud Storage
service in [CO](https://www.vmware.com/topics/glossary/content/container-orchestration.html) systems, such as
Kubernetes, to obtain stateful application deployment with ease.

This is an independent fork of the official UpCloud CSI driver, maintained separately with a focus on:

- **New features** — online volume expansion, and other enhancements not yet available upstream.
- **Hardened security** — minimal runtime image, reduced attack surface, no superfluous binaries, quicker security updates.
- **Reproducible builds** — deterministic multistage container builds pinned to specific base image and Github action versions.
- **Comprehensive testing** — expanded unit, integration, and end-to-end coverage with parallel test execution.
- **Fast iteration** — active development with frequent releases; issues and PRs are addressed promptly.

Additional info about the CSI can be found
in [Kubernetes CSI Developer Documentation](https://kubernetes-csi.github.io/docs/)
and [Kubernetes Blog](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/).

## Roadmap

- **Helm chart** — package the Kubernetes manifests into a installable chart with configurable parameters.
- **Container image signing** — sign container images with Cosign and verify at deployment time.
- **Dependency scanning** — automated vulnerability scanning of Go dependencies and container images in CI.
- **Multi-arch images** — build and publish container images for `linux/arm64` in addition to `linux/amd64`.

## Deployment

### Kubernetes
Kubernetes deployment [README](deploy/kubernetes/README.md) describes how to deploy UpCloud CSI driver using `kubectl` and sidecar containers.

## Developing the CSI driver

See [DEVELOPING.md](DEVELOPING.md) for more instructions how to develop and debug UpCloud CSI driver.

## Contribution

Feel free to open PRs and issues, as the development of CSI driver is in progress.
