## Features

### Online Block Volume Expansion (ReadWriteOnce)
Resize a PVC while a pod is actively using it — no restart required. Both `ext4` and `XFS` filesystems are supported.

### File Storage (ReadWriteMany)
Dynamic provisioning for UpCloud File Storage (NFS) services. Use the `upcloud-file-storage` or `upcloud-file-storage-encrypted`
StorageClass to automatically provision a FileStorage with a default NFS share at `/share-1`.

Architecture:

```
>=1 pods -> 1 PVC -> upcloud-file-storage SC -> CSI driver -> UpCloud FileStorage
```

The share's ACL is automatically configured to allow all IP addresses read-write access. For encrypted volumes,
use the `upcloud-file-storage-encrypted` StorageClass.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-file-storage-claim
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 250Gi
  storageClassName: upcloud-file-storage
```

| Operation | Status |
|-----------|--------|
| Create (dynamic provisioning) | ✅ |
| Delete | ✅ |
| List | ✅ |
| Validate capabilities | ✅ |
| Expand (resize) | ✅ |
| ControllerPublish / Unpublish | ✅ (no-op for NFS) |

### NFS Mount Options

The CSI driver applies optimized mount options for all File Storage volumes as documented on the UpCloud website:

- `vers=4.1` — NFS version 4.1 (required for pNFS parallel I/O)
- `nconnect=8` — 8 concurrent connections for higher throughput
- `rsize=1048576` — 1 MiB read request size
- `wsize=1048576` — 1 MiB write request size
- `noatime` — skip access-time updates, reduces metadata operations
- `hard` — indefinitely retry operations after server reboot (prevents silent data loss)

These match UpCloud's recommended mount options and are applied automatically by the node plugin.

### Helm Chart
Full-featured Helm chart, published as an OCI artifact to `ghcr.io/upcloud-tools/charts`. Includes:

- Controller StatefulSet with 4 sidecars (provisioner, attacher, resizer, snapshotter)
- Node DaemonSet with node-driver-registrar
- Snapshot controller (2 replicas, leader election) and optional validation webhook backed by cert-manager
- StorageClasses for all three UpCloud tiers (`maxiops`, `standard`, `hdd`) and NFS (encrypted)
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