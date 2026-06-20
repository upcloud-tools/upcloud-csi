TAG ?= $(shell git describe --tags)
COMMIT = $(shell git log --format="%h" -n 1)
TREE_STATE = $(shell git diff --quiet && echo 'clean' || echo 'dirty')

CONTAINER_REPO ?= ghcr.io/upcloud-tools/upcloud-csi-test
IMAGE_TAG ?= $(shell git rev-parse HEAD)


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

.PHONY: helm-install
helm-install:
	helm install upcloud-csi $(HELM_CHART_DIR) --namespace kube-system \
		$(if $(HELM_VALUES),--values $(HELM_VALUES),) \
		$(HELM_OPTS) --wait --timeout 180s

.PHONY: helm-upgrade
helm-upgrade:
	helm upgrade upcloud-csi $(HELM_CHART_DIR) --namespace kube-system \
		$(if $(HELM_VALUES),--values $(HELM_VALUES),) \
		$(HELM_OPTS) --wait --timeout 180s

.PHONY: deploy-test-sc
deploy-test-sc:
	kubectl apply -f test/e2e/test-storage-classes.yaml

.PHONY: clean-tests
clean-tests:
	kubectl -n "$(NAMESPACE)" patch volumesnapshots.snapshot.storage.k8s.io --all \
		-p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
	kubectl delete namespace "$(NAMESPACE)" --timeout=30s 2>/dev/null || true
	kubectl patch volumesnapshotcontents.snapshot.storage.k8s.io \
		-l "csi-test=$(TEST_RUN_ID)" \
		-p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
	kubectl delete volumesnapshotcontents.snapshot.storage.k8s.io \
		-l "csi-test=$(TEST_RUN_ID)" --ignore-not-found --timeout=30s
	kubectl delete volumesnapshotclasses.snapshot.storage.k8s.io \
		-l "csi-test=$(TEST_RUN_ID)" --ignore-not-found --timeout=30s

.PHONY: test
test:
	go vet ./...
	go test -race ./...

test-integration:
	make -C test/integration test

# CI-friendly e2e test — used in matrix strategy where each job runs one test.
# Use named shortcuts to run a single test case:
#   make test-e2e SNAPSHOT=y       — Create Snapshot And Restore
#   make test-e2e RESIZE=y         — Resize Volume (ext4 + xfs, sequential)
#   make test-e2e RESIZE_EXT4=y    — Resize Volume (ext4 only)
#   make test-e2e RESIZE_XFS=y     — Resize Volume (xfs only)
#   make test-e2e LIST=y           — List Volumes
#   make test-e2e PERSISTENCE=y    — Attach Detach Volume
#   make test-e2e CREATEDELETE=y   — Create Delete Volume
.PHONY: test-e2e
test-e2e:
	@echo "==> Running e2e tests"
	cd test/e2e && go test -tags e2e -v -timeout 30m \
		$(if $(CREATEDELETE),--ginkgo.focus="Create Delete Volume",) \
		$(if $(LIST),--ginkgo.focus="List Volumes",) \
		$(if $(RESIZE_EXT4),--ginkgo.focus="Resize Volume$$",) \
		$(if $(RESIZE_XFS),--ginkgo.focus="Resize Volume XFS",) \
		$(if $(RESIZE),--ginkgo.focus="Resize Volume",) \
		$(if $(PERSISTENCE),--ginkgo.focus="Attach Detach Volume",) \
		$(if $(SNAPSHOT),--ginkgo.focus="Create Snapshot And Restore",) \
		./...

# Local-development variant — sequential execution with real-time output.
# Use named shortcuts to run a single test case:
#   make test-e2e-verbose SNAPSHOT=y      — Create Snapshot And Restore
#   make test-e2e-verbose RESIZE=y        — Resize Volume (ext4 + xfs, sequential)
#   make test-e2e-verbose RESIZE_EXT4=y   — Resize Volume (ext4 only)
#   make test-e2e-verbose RESIZE_XFS=y    — Resize Volume (xfs only)
#   make test-e2e-verbose LIST=y          — List Volumes
#   make test-e2e-verbose PERSISTENCE=y   — Attach Detach Volume
#   make test-e2e-verbose CREATEDELETE=y  — Create Delete Volume
.PHONY: test-e2e-verbose
test-e2e-verbose:
	@echo "==> Running e2e tests (verbose mode)"
	cd test/e2e && go test -tags e2e -v --ginkgo.output-interceptor-mode=none -timeout 30m \
		$(if $(CREATEDELETE),--ginkgo.focus="Create Delete Volume",) \
		$(if $(LIST),--ginkgo.focus="List Volumes",) \
		$(if $(RESIZE_EXT4),--ginkgo.focus="Resize Volume$$",) \
		$(if $(RESIZE_XFS),--ginkgo.focus="Resize Volume XFS",) \
		$(if $(RESIZE),--ginkgo.focus="Resize Volume",) \
		$(if $(PERSISTENCE),--ginkgo.focus="Attach Detach Volume",) \
		$(if $(SNAPSHOT),--ginkgo.focus="Create Snapshot And Restore",) \
		./...

.PHONY: release-notes
release-notes: CHANGELOG_HEADER = ^\#\# \[
release-notes: CHANGELOG_VERSION = $(subst v,,$(TAG))
release-notes:
	@awk \
		'/${CHANGELOG_HEADER}${CHANGELOG_VERSION}/ { flag = 1; next } \
		/${CHANGELOG_HEADER}/ { if ( flag ) { exit; } } \
		flag { if ( n ) { print prev; } n++; prev = $$0 }' \
		CHANGELOG.md

HELM_CHART_VERSION = $(or $(CHART_VERSION),$(shell awk '/^version:/ {print $$2}' deploy/helm/Chart.yaml))
.PHONY: helm-release-notes
helm-release-notes:
	@awk \
		'/^## \['$(HELM_CHART_VERSION)'\]/ { flag = 1; next } \
		/^## \[/ { if ( flag ) { exit; } } \
		flag { if ( n ) { print prev; } n++; prev = $$0 }' \
		deploy/helm/CHANGELOG.md

HELM_CHART_DIR = deploy/helm

.PHONY: helm-unittest
helm-unittest:
	helm plugin install https://github.com/helm-unittest/helm-unittest.git 2>/dev/null || true
	helm unittest $(HELM_CHART_DIR)

.PHONY: helm-lint
helm-lint:
	helm lint $(HELM_CHART_DIR)

.PHONY: helm-package
helm-package:
	helm package $(HELM_CHART_DIR) --destination ./dist

.PHONY: kube-lint
kube-lint:
	kube-linter lint --config $(HELM_CHART_DIR)/.kube-linter.yaml $(HELM_CHART_DIR)

.PHONY: k8s-lint
k8s-lint:
	helm template test-release $(HELM_CHART_DIR) > /tmp/upcloud-csi-rendered.yaml
	@if command -v kubeconform > /dev/null 2>&1; then \
		kubeconform /tmp/upcloud-csi-rendered.yaml; \
	else \
		echo "kubeconform not installed. Install from https://github.com/yannh/kubeconform"; \
		exit 1; \
	fi
