# Developing the CSI driver

CSI driver's primary goal is to conform to [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec/blob/6bdbaa0472f5a1dc0e0e1f3738c65b4cac951d1f/spec.md) specification by implementing required gRPC endpoints. Unsupported endpoints should return an `CALL_NOT_IMPLEMENTED` error.  
Depending on [CO](https://www.vmware.com/topics/glossary/content/container-orchestration.html), endpoints are called directly or by sidecar containers (see [values.yaml](deploy/helm/values.yaml) for available sidecars).

## Requirements
- [Go](https://golang.org/doc/install) >= 1.26
- [Buildah](https://buildah.io/) — required for `make container-build`

Get the source code:

```shell
$ git clone git@github.com:upcloud-tools/upcloud-csi.git
$ cd upcloud-csi
```

### Building the plugin

**Container image** (recommended for testing):
```shell
$ make container-build
```
This builds the plugin inside a multistage Containerfile: Go compilation in a `golang:1.26` image, then the binary is copied into an `alpine:3.23` runtime image with all required filesystem tools.

**Local binary** (for quick iteration):
```shell
$ go build -o cmd/upcloud-csi-plugin/upcloud-csi-plugin ./cmd/upcloud-csi-plugin
```

## Project layout
### Applications
Project's application can be found under `cmd` directory:
- `upcloud-csi-plugin` is monolith CSI driver that can be run as controller or node driver (or both).

### Plugin
Required CSI interfaces are implemented in `controller`, `node` and `ìdentity` packages. 
Plugin's gRPC server uses these packages to expose endpoints described in following interfaces:
- [csi.IdentityServer](https://pkg.go.dev/github.com/container-storage-interface/spec@v1.6.0/lib/go/csi#IdentityServer)
- [csi.ControllerServer](https://pkg.go.dev/github.com/container-storage-interface/spec@v1.6.0/lib/go/csi#ControllerServer)
- [csi.NodeServer](https://pkg.go.dev/github.com/container-storage-interface/spec@v1.6.0/lib/go/csi#NodeServer)


## Testing
Run tests using `make`
```shell
$ make test
```
### Integration test (CSI sanity suite)
Runs the CSI [sanity test suite](https://github.com/kubernetes-csi/csi-test) against a running driver instance. Requires UpCloud API credentials.

```shell
$ make test-integration
```

## Logging
Driver uses structured logging which level can be set using `--log-level` flag. Only errors are logged by default. OS level commands are logged using `DEBUG` level which also logs gRPC request and response objects. Debug level is only suitable for debugging purposes.  
Logging keys are defined in [driver/log.go](driver/log.go) to keep keys consistent across driver.  
Correlation ID (`correlation_id`) is attached to log messages using request interceptor (aka middleware) so that driver operations can be tracked across controller and node.

## Tooling
CSI driver's controller functionality can be tested locally but node functions requires that driver is run in UpCloud VM so that driver can see attached disks. 

Following example commands expects that driver is running and using endpoint `/tmp/csi.sock` e.g:
```shell
$ upcloud-csi-plugin --username=$UPCLOUD_USERNAME --password=$UPCLOUD_PASSWORD --nodehost=$HOSTNAME --endpoint=unix:///tmp/csi.sock --log-level=debug
```

### Sanity Test Command Line Program
[Sanity Test](https://github.com/kubernetes-csi/csi-test/tree/master/cmd/csi-sanity) is the command line program that tests a CSI driver using the [sanity](https://github.com/kubernetes-csi/csi-test/tree/master/pkg/sanity) package test suite.
```shell
$ csi-sanity --csi.endpoint=/tmp/csi.sock --ginkgo.fail-fast -csi.testnodevolumeattachlimit
```

Focus only specs that match regular expression
```shell
$ csi-sanity --csi.endpoint=/tmp/csi.sock --ginkgo.fail-fast -csi.testnodevolumeattachlimit --ginkgo.focus ListSnapshots
```

### Container Storage Client
The [Container Storage Client (csc)](https://github.com/rexray/gocsi/tree/master/csc) is a command line interface (CLI) tool that provides analogues for all of the CSI RPCs.
Print command help
```shell
$ csc -e unix:///tmp/csi.sock help
```
Get controller capabilities
```shell
$ csc -e unix:///tmp/csi.sock controller get-capabilities
&{type:CREATE_DELETE_VOLUME }
&{type:PUBLISH_UNPUBLISH_VOLUME }
&{type:LIST_VOLUMES }
&{type:CREATE_DELETE_SNAPSHOT }
&{type:LIST_SNAPSHOTS }
&{type:EXPAND_VOLUME }
&{type:CLONE_VOLUME }
```

## Releasing

### App release

1. Update `appVersion` in `deploy/helm/Chart.yaml` to the new version.
   Optionally bump `version` as well.
2. Add a changelog entry to root `CHANGELOG.md` under the new version header.
3. Tag and push — CI validates the Chart versions were bumped and matches `appVersion` to the tag.
   ```shell
   git tag v2.6.0
   git push origin v2.6.0
   ```

### Helm chart release

Bump when templates or values change independently of an app release.

1. Update `version` in `deploy/helm/Chart.yaml`.
2. Update `deploy/helm/CHANGELOG.md` and the `artifacthub.io/changes` annotation in `Chart.yaml`.
3. Package and publish the chart to your OCI registry or chart repo.
4. Tag and push:
   ```shell
   git tag chart-v1.0.1
   git push origin chart-v1.0.1
   ```
   The CI workflow creates a draft release with notes from `deploy/helm/CHANGELOG.md`.

