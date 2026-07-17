## Features

### Online Block Volume Expansion (ReadWriteOnce)
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