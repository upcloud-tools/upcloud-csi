# UpCloud CSI Driver
![GitHub Actions status](https://github.com/upcloud-tools/upcloud-csi/actions/workflows/build.yaml/badge.svg)

## Overview

UpCloud [CSI](https://github.com/container-storage-interface/spec) Driver provides support for UpCloud Block Storage in
Kubernetes.

This is an **independent** fork of the official UpCloud CSI driver, maintained separately with a focus on:

- **New features** — online volume expansion, Helm chart and other enhancements not yet available upstream.
- **Hardened security** — minimal runtime image, reduced attack surface, no superfluous binaries, quicker security updates.
- **Reproducible builds** — deterministic multistage container builds pinned to specific base image and Github action versions.
- **Comprehensive testing** — expanded unit, integration, and end-to-end coverage.
- **Fast iteration** — active development with frequent releases; issues and PRs are addressed promptly.

Additional info about the CSI can be found
in [Kubernetes CSI Developer Documentation](https://kubernetes-csi.github.io/docs/)
and [Kubernetes Blog](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/).

## Roadmap

- **Container image signing** — sign container images with Cosign and verify at deployment time.
- **Dependency scanning** — automated vulnerability scanning of Go dependencies and container images in CI.
- **Multi-arch images** — build and publish container images for `linux/arm64` in addition to `linux/amd64`.

## Deployment

### Helm chart

```shell
helm install upcloud-csi oci://ghcr.io/upcloud-tools/helm-charts/upcloud-csi \
  --namespace kube-system --version 1.0.0 \
  --set credentials.username=YOUR_USERNAME \
  --set credentials.password=YOUR_PASSWORD
```

If the `upcloud` secret already exists in the namespace, omit the credentials:

```shell
helm install upcloud-csi oci://ghcr.io/upcloud-tools/helm-charts/upcloud-csi \
  --namespace kube-system --version 1.0.0 \
  --set credentials.createSecret=false
```

All values have sensible defaults. See [values.yaml](deploy/helm/values.yaml) for the full reference.

To customize, create a values file and pass it with `--values`:

```shell
helm install upcloud-csi oci://ghcr.io/upcloud-tools/helm-charts/upcloud-csi \
  --namespace kube-system --version 1.0.0 --values my-values.yaml
```

## Developing the CSI driver

See [DEVELOPING.md](DEVELOPING.md) for more instructions how to develop and debug UpCloud CSI driver.

