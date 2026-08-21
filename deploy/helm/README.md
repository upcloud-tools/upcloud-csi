# upcloud-csi

The UpCloud [CSI](https://github.com/container-storage-interface/spec) driver, packaged as a Helm chart.
It provides UpCloud Block Storage with online volume resizing, ReadWriteMany File Storage (NFS) to Kubernetes, plus snapshot support.

## Prerequisites

- A Kubernetes cluster running **Kubernetes >= 1.34** (`kubeVersion: ">=1.34.0"`).
- An [UpCloud](https://upcloud.com) account with an API token that can manage block/file storage.
- Optionally, for the snapshot validation webhook:
  - a manually created TLS `Secret`, or
  - [cert-manager](https://cert-manager.io) installed (when `snapshotValidationWebhook.certManager.enabled` is `true`).
- Optionally, the [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator) CRDs (when `metrics.serviceMonitor.enabled` or `metrics.prometheusRule.enabled` is `true`).

## Installing

The chart is published to the UpCloud Tools OCI registry:

```sh
helm install upcloud-csi oci://ghcr.io/upcloud-tools/charts/upcloud-csi \
  --namespace kube-system \
  --set credentials.token=<UPCLOUD_API_TOKEN>
```

To upgrade later:

```sh
helm upgrade upcloud-csi oci://ghcr.io/upcloud-tools/charts/upcloud-csi \
  --namespace kube-system \
  --set credentials.token=<UPCLOUD_API_TOKEN>
```

> [!NOTE]
> Credentials are supplied either by creating a `Secret` from `credentials.token` (`credentials.createSecret: true`),
> or by referencing an existing `Secret` via `credentials.secretName` (the default secret name is `upcloud`).
> `credentials.createSecret` is `false`, the referenced secret must already exist.

## Configuration

### Global

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `clusterZone` | string | `""` | UpCloud zone (e.g. `de-fra1`). Auto-discovered when empty. |
| `logLevel` | int | `5` | Default log verbosity (0–10). |
| `credentials.createSecret` | bool | `false` | Create a `Secret` from `credentials.token`. |
| `credentials.secretName` | string | `upcloud` | Name of the credentials `Secret` (created or referenced). |
| `credentials.tokenKey` | string | `token` | Key inside the `Secret` holding the UpCloud API token. |
| `credentials.token` | string | `""` | UpCloud API token (required when `credentials.createSecret` is `true`). |
| `csiDriver.name` | string | `storage.csi.upcloud.com` | Name of the `CSIDriver` resource. |
| `csiDriver.attachRequired` | bool | `true` | Whether attach is required by the driver. |
| `csiDriver.podInfoOnMount` | bool | `true` | Pass pod info to the node service on mount. |
| `csiDriver.fsGroupPolicy` | string | `File` | `fsGroupPolicy`: `File`, `None`, or `ReadWriteOnceWithFSType`. |
| `image.repository` | string | `ghcr.io/upcloud-tools/upcloud-csi` | Driver image repository. |
| `image.tag` | string | `""` | Driver image tag (defaults to the chart `appVersion` when empty). |
| `image.pullPolicy` | string | `IfNotPresent` | Driver image pull policy. |
| `storageClasses.enabled` | bool | `false` | Create the bundled `StorageClass`es. |
| `volumeSnapshotClass.enabled` | bool | `true` | Create the `VolumeSnapshotClass`. |
| `metrics.enabled` | bool | `true` | Expose metrics endpoints and the metrics `Service`. |
| `metrics.serviceMonitor.enabled` | bool | `false` | Create a `ServiceMonitor` (requires prometheus-operator). |
| `metrics.prometheusRule.enabled` | bool | `false` | Create a `PrometheusRule`. |
| `networkPolicy.enabled` | bool | `false` | Create `NetworkPolicy` resources restricting pod communication. |
| `nameOverride` | string | `""` | Override the chart name. |
| `fullnameOverride` | string | `""` | Override the fully qualified resource name. |
| `commonLabels` | object | `{}` | Common labels added to all resources. |
| `extraObjects` | list | `[]` | Extra Kubernetes objects to deploy (supports Go template expressions). |

### Per-component settings

The `controller`, `node`, `snapshotController`, and `snapshotValidationWebhook` sections share the following common keys (defaults shown where fixed):

| Key (under each component) | Type | Description |
| --- | --- | --- |
| `logLevel` | int/null | Component log verbosity (inherits global `logLevel` when `null`). |
| `replicas` | int | Replica count (controller `1`, snapshot controller `2`). |
| `imagePullSecrets` | list | Image pull secrets for the component's pods. |
| `resources` | object | `requests`/`limits` for the driver container. |
| `serviceAccountName` | string | SA name (defaults to `{fullname}-<component>`). |
| `serviceAccountAnnotations` | object | Annotations for the component's `ServiceAccount`. |
| `priorityClassName` | string | `system-cluster-critical` (controller/snapshot/webhook) or `system-node-critical` (node). |
| `podLabels` / `podAnnotations` | object | Extra pod template labels/annotations. |
| `nodeSelector` / `tolerations` / `affinity` | various | Pod scheduling constraints. |
| `updateStrategy` | object | Rolling update strategy. |
| `minReadySeconds` / `revisionHistoryLimit` | int | Workload rollout tuning. |
| `terminationGracePeriodSeconds` | int | Pod termination grace period. |
| `lifecycle` | object | Container lifecycle hooks. |
| `topologySpreadConstraints` | list | Topology spread constraints. |
| `runtimeClassName` / `dnsPolicy` / `dnsConfig` / `hostAliases` | various | Advanced pod settings. |
| `initContainers` / `additionalVolumes` / `additionalVolumeMounts` | various | Pod customization. |
| `securityContext` / `podSecurityContext` | object | Container/pod security contexts. |
| `podDisruptionBudget.enabled` | bool | Enable a PDB for the component. |
| `podDisruptionBudget.maxUnavailable` / `minAvailable` | int/string | PDB thresholds. |

Component-specific keys worth noting:

- `controller.sidecars` — image/tag/pullPolicy/resources for `csiProvisioner`, `csiAttacher`, `csiResizer`, and `csiSnapshotter`.
- `node.sidecars.csiNodeDriverRegistrar` — node registrar image settings.
- `snapshotController.image`, `snapshotController.replicas` (`2`).
- `snapshotValidationWebhook` — `enabled` (requires a TLS `Secret` or cert-manager),
  `tlsSecretName`, `caBundle`, `certManager.{enabled,issuerName,issuerKind}`, `timeoutSeconds`, and `failurePolicy` (`Ignore`/`Fail`).
- `storageClasses.classes` — list of storage classes (block tiers: `maxiops`, `hdd`, `standard`; file/NFS: `nfs` with optional `encryption: data-at-rest`).

For the full set of options and inline documentation, see `values.yaml` and `values.schema.json` in the chart.
