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

### Online Volume Expansion
Resize a PVC while a pod is actively using it — no restart required. Both `ext4` and `XFS` filesystems are supported.

### NFS File Storage (ReadWriteMany) — **BETA**
Static provisioning for UpCloud NFS File Storage services. Create a FileStorage manually, then mount it as a `ReadWriteMany` volume across multiple pods.

Architecture:

```
>=1 pods -> 1 PVC -> 1 PV -> NFS File Storage with 1 share path
```
**Note**: Just a single share path is supported in this approach. Multiple PVs using the same File Storage service (and different share paths) is currently not supported.

Original docs: https://upcloud.com/docs/guides/file-storage-nfs-managed-kubernetes/

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: my-file-storage-pv
spec:
  capacity:
    storage: 250Gi
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: "" # Prevents dynamic provisioning
  nfs:
    server: FILE_STORAGE_IP
    path: /your/share/path
  mountOptions:
    - vers=4.1
    - nconnect=8
    - rsize=1048576
    - wsize=1048576
    - noatime
    - hard
```

The CSI driver currently supports delete, list, validate capabilities, and expand for existing FileStorage volumes.
`CreateVolume` is not implemented for FileStorage (dynamic provisioning is not available). FileStorage services must
be created directly via UpCloud or the CLI.

| Operation | Status |
|-----------|--------|
| Create (dynamic provisioning) | ❌ Not implemented |
| Delete | ✅ |
| List | ✅ |
| Validate capabilities | ✅ |
| Expand (resize) | ✅ |
| ControllerPublish / Unpublish | ❌ Not called for `nfs:` PVs (kubelet handles mount) |

### Helm Chart
Full-featured Helm chart, published as an OCI artifact to `ghcr.io/upcloud-tools/charts`. Includes:

- Controller StatefulSet with 4 sidecars (provisioner, attacher, resizer, snapshotter)
- Node DaemonSet with node-driver-registrar
- Snapshot controller (2 replicas, leader election) and optional validation webhook backed by cert-manager
- StorageClasses for all three UpCloud tiers: `maxiops`, `standard`, `hdd`
- `securityContext` and `podSecurityContext` per component with secure defaults
- `metrics` block — ClusterIP metrics Service, optional ServiceMonitor and PrometheusRule for prometheus-operator
- `extraObjects` — deploy arbitrary Kubernetes resources with Go template support
- Configurable pod spec fields including `imagePullSecrets` and health probes
- PodDisruptionBudget support for controller and snapshot-controller
- NetworkPolicy support for in-cluster traffic isolation
- Token-based Bearer auth (`credentials.token` + `credentials.tokenKey`) for new deployments
- Resource labels and annotations, configurable per component
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
