PLUGIN_NAME=upcloud-csi-plugin
PLUGIN_PKG ?= github.com/upcloud-tools/upcloud-csi
PLUGIN_CMD ?= ${PLUGIN_PKG}/cmd/upcloud-csi-plugin
OS ?= linux
GO_VERSION := 1.22
ARCH := amd64
CGO_ENABLED := 1
TAG ?= $(shell git describe --tags)
COMMIT = $(shell git log --format="%h" -n 1)
TREE_STATE = $(shell git diff --quiet && 'clean' || echo 'dirty')
LDFLAGS ?= "-s -w -X ${PLUGIN_PKG}/internal/plugin.version=${TAG} \
-X ${PLUGIN_PKG}/internal/plugin.commit=${COMMIT} \
-X ${PLUGIN_PKG}/internal/plugin.gitTreeState=${TREE_STATE}"

CONTAINER_REPO ?= ghcr.io/upcloud-tools/upcloud-csi
IMAGE_TAG  ?= $(shell git rev-parse --short HEAD)

.PHONY: compile
compile:
	@echo "==> Building the project"
	@docker run --rm -e CGO_ENABLED=${CGO_ENABLED} -e GOOS=${OS} -e GOARCH=${ARCH} -v ${PWD}/:/app -w /app golang:${GO_VERSION}-alpine sh -c \
		'apk add git make build-base && \
		go build -buildvcs=false -ldflags ${LDFLAGS} -o cmd/upcloud-csi-plugin/${PLUGIN_NAME} ${PLUGIN_CMD}'


.PHONY: container-build
container-build:
	buildah build --platform linux/amd64 -t $(CONTAINER_REPO):$(IMAGE_TAG) cmd/upcloud-csi-plugin -f cmd/upcloud-csi-plugin/Containerfile

.PHONY: push-image
push-image: build-plugin
	@echo "==> Building and pushing image $(CONTAINER_REPO):$(IMAGE_TAG)"
	buildah build --platform linux/amd64 -t $(CONTAINER_REPO):$(IMAGE_TAG) -f cmd/upcloud-csi-plugin/Containerfile cmd/upcloud-csi-plugin
	buildah push $(CONTAINER_REPO):$(IMAGE_TAG)

.PHONY: deploy-manifests
deploy-manifests:
	@echo "==> Deploying CSI driver manifests (image: $(CONTAINER_REPO):$(IMAGE_TAG))"
	KUBECONFIG=$(KUBECONFIG) kubectl apply -f deploy/kubernetes/crd-upcloud-csi.yaml
	KUBECONFIG=$(KUBECONFIG) kubectl apply -f deploy/kubernetes/rbac-upcloud-csi.yaml
	sed 's|ghcr.io/upcloudltd/upcloud-csi:latest|$(CONTAINER_REPO):$(IMAGE_TAG)|g' \
		deploy/kubernetes/setup-upcloud-csi.yaml | KUBECONFIG=$(KUBECONFIG) kubectl apply -f -
	KUBECONFIG=$(KUBECONFIG) kubectl -n kube-system rollout status statefulset/csi-upcloud-controller --timeout=180s
	KUBECONFIG=$(KUBECONFIG) kubectl -n kube-system rollout status daemonset/csi-upcloud-node --timeout=180s

.PHONY: clean-tests
clean-tests:
	KUBECONFIG=$(KUBECONFIG) kubectl delete all --all
	KUBECONFIG=$(KUBECONFIG) kubectl delete persistentvolumeclaims --all

.PHONY: test
test:
	go vet ./...
	go test -race ./...

test-integration:
	make -C test/integration test

.PHONY: test-e2e
test-e2e:
	@echo "==> Running e2e tests"
	cd test/e2e && go test -tags e2e -v -timeout 30m ./...

build-plugin:
	CGO_ENABLED=0 go build -ldflags ${LDFLAGS} -o cmd/upcloud-csi-plugin/${PLUGIN_NAME} ${PLUGIN_CMD}

.PHONY: build
build: build-plugin

.PHONY: release-notes
release-notes: CHANGELOG_HEADER = ^\#\# \[
release-notes: CHANGELOG_VERSION = $(subst v,,$(TAG))
release-notes:
	@awk \
		'/${CHANGELOG_HEADER}${CHANGELOG_VERSION}/ { flag = 1; next } \
		/${CHANGELOG_HEADER}/ { if ( flag ) { exit; } } \
		flag { if ( n ) { print prev; } n++; prev = $$0 }' \
		CHANGELOG.md
