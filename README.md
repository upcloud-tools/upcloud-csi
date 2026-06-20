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

## Repository Security

This repository uses the following security and supply-chain measures:

| Practice | Implementation |
|---|---|---|
| **Security policy** | `SECURITY.md` directs reporters to GitHub's Private vulnerability reporting tool. |
| **Vulnerability reporting** | Private vulnerability reporting enabled — reporters get an acknowledgment within 72 hours. |
| **Code scanning (CodeQL)** | `github/codeql-action` analyzes Go code on every push/PR to `main` and weekly. |
| **Dependabot alerts** | Dependabot monitors Go modules, GitHub Actions, and Docker dependencies daily; alerts enabled for vulnerable dependencies. |
| **Secret scanning** | GitHub's built-in secret scanning alerts enabled at the repository level. |
| **Branch protection** | `main` requires passing status checks (`golangci-lint`, `helm-lint`, `test`, CodeQL) and pull request review before merge. Admins can bypass. |
| **Action pinning** | All GitHub Actions are pinned by commit SHA with a human-readable version comment. The repo enforces SHA pinning globally. |
| **Static analysis** | `golangci-lint` with 50+ linters (`gosec`, `staticcheck`, `errcheck`, etc.) runs on every PR. |
| **Container image** | Distroless-inspired Alpine runtime, multistage build, pinned base image versions. |
| **Release integrity** | Helm chart validates that `appVersion` matches the git tag and that the container image exists before publishing. |
| **Artifact Hub** | Helm chart metadata published to Artifact Hub for discoverability. |

## Deployment

### Helm chart

```shell
helm install upcloud-csi oci://ghcr.io/upcloud-tools/charts/upcloud-csi \
  --namespace kube-system --version 1.0.0 \
  --set credentials.username=YOUR_USERNAME \
  --set credentials.password=YOUR_PASSWORD
```

If the `upcloud` secret already exists in the namespace, omit credentials:

```shell
helm install upcloud-csi oci://ghcr.io/upcloud-tools/charts/upcloud-csi \
  --namespace kube-system --version 1.0.0 \
  --set credentials.createSecret=false
```

All values have sensible defaults. See [values.yaml](deploy/helm/values.yaml) for the full reference. To customize,
create a values file and pass it with `--values`:

```shell
helm install upcloud-csi oci://ghcr.io/upcloud-tools/charts/upcloud-csi \
  --namespace kube-system --version 1.0.0 --values my-values.yaml
```

## Developing

See [DEVELOPING.md](DEVELOPING.md) for instructions on how to develop and debug the UpCloud CSI driver.
