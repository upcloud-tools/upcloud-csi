TAG ?= $(shell git describe --tags)
COMMIT = $(shell git log --format="%h" -n 1)
TREE_STATE = $(shell git diff --quiet && echo 'clean' || echo 'dirty')

CONTAINER_REPO ?= ghcr.io/upcloud-tools/upcloud-csi-test
IMAGE_TAG  ?= $(shell git rev-parse --short HEAD)


.PHONY: container-build
container-build:
	buildah build --platform linux/amd64 \
		--build-arg VERSION=$(TAG) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg TREE_STATE=$(TREE_STATE) \
		-t $(CONTAINER_REPO):$(IMAGE_TAG) \
		-f cmd/upcloud-csi-plugin/Containerfile .

.PHONY: push-image
push-image: container-build
	@echo "==> Pushing image $(CONTAINER_REPO):$(IMAGE_TAG)"
	buildah push $(CONTAINER_REPO):$(IMAGE_TAG)

.PHONY: deploy-manifests
deploy-manifests:
	@echo "==> Deploying CSI driver manifests (image: $(CONTAINER_REPO):$(IMAGE_TAG))"
	kubectl apply -f deploy/kubernetes/crd-upcloud-csi.yaml
	kubectl apply -f deploy/kubernetes/rbac-upcloud-csi.yaml
	sed 's|ghcr.io/upcloudltd/upcloud-csi:latest|$(CONTAINER_REPO):$(IMAGE_TAG)|g' \
		deploy/kubernetes/setup-upcloud-csi.yaml | kubectl apply -f -
	kubectl apply -f deploy/kubernetes/sc-upcloud-csi-test.yaml
	kubectl -n kube-system rollout status statefulset/csi-upcloud-controller --timeout=180s
	kubectl -n kube-system rollout status daemonset/csi-upcloud-node --timeout=180s

.PHONY: clean-tests
clean-tests:
	kubectl -n default delete all --all
	kubectl -n default delete persistentvolumeclaims --all
	kubectl delete storageclass upcloud-block-storage-maxiops-test --ignore-not-found

.PHONY: test
test:
	go vet ./...
	go test -race ./...

test-integration:
	make -C test/integration test

# CI-friendly e2e test — Ginkgo output interceptor captures subprocess output and emits it per test case.
.PHONY: test-e2e
test-e2e:
	@echo "==> Running e2e tests"
	cd test/e2e && go test -tags e2e -v -timeout 30m ./...

# Local-development variant — disables Ginkgo's output interceptor logs appear in real time.
# Do not use in CI: output from different test cases can interleave.
.PHONY: test-e2e-verbose
test-e2e-verbose:
	@echo "==> Running e2e tests (verbose mode)"
	cd test/e2e && go test -tags e2e -v --ginkgo.output-interceptor-mode=none -timeout 30m ./...

.PHONY: release-notes
release-notes: CHANGELOG_HEADER = ^\#\# \[
release-notes: CHANGELOG_VERSION = $(subst v,,$(TAG))
release-notes:
	@awk \
		'/${CHANGELOG_HEADER}${CHANGELOG_VERSION}/ { flag = 1; next } \
		/${CHANGELOG_HEADER}/ { if ( flag ) { exit; } } \
		flag { if ( n ) { print prev; } n++; prev = $$0 }' \
		CHANGELOG.md
