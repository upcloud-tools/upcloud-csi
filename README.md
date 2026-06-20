# UpCloud CSI Driver
![Build](https://github.com/upcloud-tools/upcloud-csi/actions/workflows/build.yaml/badge.svg)
![Helm Lint](https://github.com/upcloud-tools/upcloud-csi/actions/workflows/lint-helm.yaml/badge.svg)
![Tests](https://github.com/upcloud-tools/upcloud-csi/actions/workflows/test.yaml/badge.svg)

UpCloud [CSI](https://github.com/container-storage-interface/spec) Driver provides support for UpCloud Block Storage in
Kubernetes.

This is an **independent** community fork of the official UpCloud CSI driver, maintained separately with a focus on
features, security, and fast iteration.

## Features

### Online Volume Expansion
Resize a PVC while a pod is actively using it — no restart required. Both `ext4` and `XFS` filesystems are supported.

### Helm Chart
Full-featured Helm chart, published as an OCI artifact to `ghcr.io/upcloud-tools/charts`. Includes:

- Controller StatefulSet with 4 sidecars (provisioner, attacher, resizer, snapshotter)
- Node DaemonSet with node-driver-registrar
- Snapshot controller (2 replicas, leader election) and optional validation webhook
- StorageClasses for all three UpCloud tiers: `maxiops`, `standard`, `hdd`
- PodDisruptionBudget support for controller and snapshot-controller
- Credential checksum annotation for automatic pod rollout on secret changes

### Volume Snapshots
Updated to `csi-snapshotter` / `snapshot-controller` / `snapshot-validation-webhook` v8.6.0 with CEL-based CRD
validation. Full E2E coverage for snapshot creation and PVC restore from snapshot.

### Minimal Runtime Image
Multistage Containerfile produces an Alpine-based image with only the packages required for block storage operations
(`xfsprogs`, `e2fsprogs`, `cloud-utils-growpart`, etc.) — no superfluous binaries.

### Prometheus Metrics
The driver exposes Prometheus metrics at `:8090/metrics` (configurable via `--metrics-address`). Includes:

- **CSI gRPC operations** — `csi_plugin_operations_total` (by method + status), `csi_plugin_operation_duration_seconds` (histogram), `csi_plugin_operations_in_flight` (gauge)
- **UpCloud API calls** — `upcloud_api_requests_total` (by method + result), `upcloud_api_request_duration_seconds` (histogram)
- **Go runtime** — goroutines, GC, memory, CPU, and file descriptor metrics

The Helm chart provides a ClusterIP metrics Service and optional `ServiceMonitor` / `PrometheusRule` resources for
prometheus-operator. Controller sidecars expose `--http-endpoint` on ports 8080–8083.

## Repository Security

This repository uses the following security and supply-chain measures:

- **Security policy** — `SECURITY.md` directs reporters to GitHub's Private vulnerability reporting tool.
- **Vulnerability reporting** — Private vulnerability reporting enabled; reporters get an acknowledgment within 72 hours.
- **Code scanning (CodeQL)** — `github/codeql-action` analyzes Go code on every push/PR to `main` and weekly. Maintainability and Reliability scores are **Excellent** (0 findings).
- **Dependabot alerts** — Monitors Go modules, GitHub Actions, and Docker dependencies daily with alerts for vulnerable dependencies.
- **Secret scanning** — GitHub's built-in secret scanning alerts enabled at the repository level.
- **Branch protection** — `main` requires passing status checks (`golangci-lint`, `helm-lint`, `test`, CodeQL) and pull request review before merge.
- **Action pinning** — All GitHub Actions pinned by commit SHA with a human-readable version comment; enforced globally.
- **Static analysis** — `golangci-lint` with 50+ linters (`gosec`, `staticcheck`, `errcheck`, etc.) runs on every PR.
- **Container image** — Distroless-inspired Alpine runtime, multistage build, pinned base image versions.
- **Release integrity** — Helm chart validates that `appVersion` matches the git tag and that the container image exists before publishing.
- **Artifact Hub** — Helm chart metadata published to Artifact Hub for discoverability.

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

Or keep the existing CRDs and install with `--skip-crds`:

### Helm chart (install/upgrade)

If the `upcloud` secret already exists in the namespace, omit the credentials (default behavior):

```shell
helm upgrade --install upcloud-csi oci://ghcr.io/upcloud-tools/charts/upcloud-csi \
  --namespace kube-system --version 1.2.0
```

Or specify credentials to create the secret (prepend with a space to avoid saving to shell history):

```shell
helm upgrade --install upcloud-csi oci://ghcr.io/upcloud-tools/charts/upcloud-csi \
  --namespace kube-system --version 1.2.0 \
  --set credentials.createSecret=true \
  --set credentials.username=YOUR_USERNAME \
  --set credentials.password=YOUR_PASSWORD
```

By default, StorageClasses are **disabled**. Enable them with `--set storageClasses.enabled=true` if you want the chart to manage them.

All values have sensible defaults. See [values.yaml](deploy/helm/values.yaml) for the full reference.

To customize, create a values file and pass it with `--values`:

```shell
helm upgrade --install upcloud-csi oci://ghcr.io/upcloud-tools/charts/upcloud-csi \
  --namespace kube-system --version 1.2.0 --values values.yaml
```

## Credits

- **UpCloud Ltd** — Sponsors the test infrastructure used for integration and e2e testing.
- **Zed Industries** — Provides a free version of their editor.

## Developing

See [DEVELOPING.md](DEVELOPING.md) for instructions on how to develop and debug the UpCloud CSI driver.
