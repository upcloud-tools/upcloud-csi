# UpCloud CSI Driver
![Build](https://github.com/upcloud-tools/upcloud-csi/actions/workflows/release-app.yaml/badge.svg)
![Go Lint](https://github.com/upcloud-tools/upcloud-csi/actions/workflows/lint-golang.yaml/badge.svg)
![Helm Lint](https://github.com/upcloud-tools/upcloud-csi/actions/workflows/lint-helm.yaml/badge.svg)
![Tests](https://github.com/upcloud-tools/upcloud-csi/actions/workflows/test.yaml/badge.svg)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/upcloud-tools/upcloud-csi/badge)](https://scorecard.dev/viewer/?uri=github.com/upcloud-tools/upcloud-csi)

UpCloud [CSI](https://github.com/container-storage-interface/spec) Driver provides support for UpCloud Block Storage in
Kubernetes.

This is an **independent** community fork of the official UpCloud CSI driver, maintained separately with a focus on
features, security, and fast iteration.

## Features

This fork contains these extra features:

- Online block volume expansion (ReadWriteOnce)
- NFS File Storage (ReadWriteMany)
- Helm chart
- Robust integration testing to ensure feature stability
- Hardened container images
- Prometheus metrics

For full feature details, see [FEATURES.md](FEATURES.md).

## Repository Security

The project is continuously scanned with [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/upcloud-tools/upcloud-csi), which evaluates branch protection, SAST tooling, pinned dependencies, CI tests, and other security best practices. Results are published as GitHub code scanning alerts and a badge in this README.

Additional security tooling:
- **CodeQL** — Go analysis on every push and PR
- **Trivy** — Container image vulnerability scanning on every release
- **Dependabot** — Automated dependency updates with weekly pull requests
- **Signed releases** — Container images are signed with cosign (keyless) via GitHub OIDC

See [at organization level](https://github.com/upcloud-tools) for the org-wide security policy.

## Deployment

> **UpCloud Kubernetes clusters** ship with the official UpCloud CSI driver pre-installed as raw manifests.
> To replace it with this fork, remove the old installation first:

```shell
kubectl delete sts csi-upcloud-controller -n kube-system --ignore-not-found
kubectl delete daemonset csi-upcloud-node -n kube-system --ignore-not-found
kubectl delete deployment csi-upcloud-snapshot-controller -n kube-system --ignore-not-found
kubectl delete csidriver storage.csi.upcloud.com --ignore-not-found
```

> **Warning:** The commands below delete VolumeSnapshots and VolumeSnapshotContents
> across all namespaces. This is a destructive operation — make sure no data depends
> on those snapshots before proceeding.

If the cluster already has VolumeSnapshot CRDs (e.g. from a previous CSI driver installation), remove them before installing this chart:

```shell
kubectl delete volumesnapshot --all --all-namespaces --ignore-not-found
kubectl delete volumesnapshotcontent --all --ignore-not-found
kubectl delete crd volumesnapshotclasses.snapshot.storage.k8s.io \
                   volumesnapshotcontents.snapshot.storage.k8s.io \
                   volumesnapshots.snapshot.storage.k8s.io
```

Or keep the existing CRDs and install with `--skip-crds` (not recommended).

### Helm chart (install/upgrade)

UpCloud Kubernetes clusters ship with an `upcloud` secret in `kube-system` by default. If the secret exists, just install:

```shell
helm upgrade --install upcloud-csi oci://ghcr.io/upcloud-tools/charts/upcloud-csi \
  --namespace kube-system --version [CHART_VERSION]
```

To have the chart create the secret instead, set `credentials.createSecret=true` and provide the credentials.
By default, StorageClasses are **disabled**. Enable them with `--set storageClasses.enabled=true`.

All values have sensible defaults. See [values.yaml](deploy/helm/values.yaml) for the full reference.

To customize, create a values file and pass it with `--values`:

```shell
helm upgrade --install upcloud-csi oci://ghcr.io/upcloud-tools/charts/upcloud-csi \
  --namespace kube-system --version [CHART_VERSION] --values values.yaml
```

Verify container image signature:

  ```shell
  cosign verify ghcr.io/upcloud-tools/upcloud-csi:[IMAGE_VERSION] \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    --certificate-identity-regexp https://github.com/upcloud-tools/upcloud-csi/.github/workflows/release-app.yaml@refs/tags/[IMAGE_VERSION]
  ```

## Credits

- **UpCloud Ltd** — Sponsors the test infrastructure used for integration and e2e testing.
- **Zed Industries** — Provides a free version of their editor.

## Developing

See [DEVELOPING.md](DEVELOPING.md) for instructions on how to develop and debug the UpCloud CSI driver.
