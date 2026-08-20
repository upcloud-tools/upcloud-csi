#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-unknown}"
COMMIT="${COMMIT:-unknown}"
TREE_STATE="${TREE_STATE:-unknown}"
TARGETARCH="${TARGETARCH:-amd64}"
IMAGE="${IMAGE:-upcloud-csi}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

ALPINE_VERSION="3.23.5"

echo "Building ${IMAGE}:${IMAGE_TAG} (arch=${TARGETARCH}, version=${VERSION})"

CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH}" \
  go build -ldflags="-s -w \
    -X github.com/upcloud-tools/upcloud-csi/internal/plugin.version=${VERSION} \
    -X github.com/upcloud-tools/upcloud-csi/internal/plugin.commit=${COMMIT} \
    -X github.com/upcloud-tools/upcloud-csi/internal/plugin.gitTreeState=${TREE_STATE}" \
    -o upcloud-csi-plugin ./cmd/upcloud-csi-plugin

CTR=$(buildah from docker.io/alpine:${ALPINE_VERSION})

buildah run "${CTR}" -- apk add --no-cache \
  ca-certificates \
  cloud-utils-growpart \
  e2fsprogs \
  e2fsprogs-extra \
  eudev \
  nfs-utils \
  parted \
  util-linux \
  xfsprogs \
  xfsprogs-extra

buildah copy "${CTR}" upcloud-csi-plugin /bin/upcloud-csi-plugin

buildah config --os linux --arch "${TARGETARCH}" "${CTR}"
buildah config --label "org.opencontainers.image.source=https://github.com/upcloud-tools/upcloud-csi" "${CTR}"
buildah config --label "org.opencontainers.image.description=UpCloud CSI Driver" "${CTR}"
buildah config --label "org.opencontainers.image.version=${VERSION}" "${CTR}"
buildah config --entrypoint '["/bin/upcloud-csi-plugin"]' "${CTR}"

buildah commit --format docker "${CTR}" "${IMAGE}:${IMAGE_TAG}"
buildah rm "${CTR}"

rm -f upcloud-csi-plugin

echo "Built ${IMAGE}:${IMAGE_TAG}"
