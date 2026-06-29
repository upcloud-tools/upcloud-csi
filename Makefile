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

.PHONY: helm-deploy
helm-deploy:
	helm upgrade --install upcloud-csi $(HELM_CHART_DIR) --namespace kube-system \
		$(if $(HELM_VALUES),--values $(HELM_VALUES),) \
		$(HELM_OPTS) --wait --timeout 180s

.PHONY: deploy-test
deploy-test:
	helm upgrade --install upcloud-csi $(HELM_CHART_DIR) --namespace kube-system \
		--set networkPolicy.enabled=true \
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

CONTROLLER_FUZZ_TARGETS = \
	FuzzValidateCreateVolumeRequest \
	FuzzValidateControllerPublishVolumeRequest \
	FuzzObtainSize \
	FuzzGetStorageRange \
	FuzzParseToken \
	FuzzDisplayByteString \
	FuzzFormatBytes \
	FuzzCreateVolumeRequestTier \
	FuzzCreateVolumeRequestEncryptionAtRest

NODE_FUZZ_TARGETS = \
	FuzzValidateNodePublishVolumeRequest

.PHONY: fuzz
fuzz:
	@for target in $(CONTROLLER_FUZZ_TARGETS); do \
		echo "==> Fuzzing $$target"; \
		go test -fuzz="^$$target$$" -fuzztime=30s ./internal/controller/; \
	done
	@for target in $(NODE_FUZZ_TARGETS); do \
		echo "==> Fuzzing $$target"; \
		go test -fuzz="^$$target$$" -fuzztime=30s ./internal/node/; \
	done

test-integration:
	make -C test/integration test


# Named shortcuts for running e2e tests locally or in CI:
#   make test-e2e-(ci/local) SNAPSHOT=y       — Create Snapshot And Restore
#   make test-e2e-(ci/local) RESIZE=y         — Resize Volume (ext4 + xfs, sequential)
#   make test-e2e-(ci/local) RESIZE_EXT4=y    — Resize Volume (ext4 only)
#   make test-e2e-(ci/local) RESIZE_XFS=y     — Resize Volume (xfs only)
#   make test-e2e-(ci/local) RESIZE_UNATTACHED=y — Resize Unattached Volume
#   make test-e2e-(ci/local) LIST=y           — List Volumes
#   make test-e2e-(ci/local) PERSISTENCE=y    — Attach Detach Volume
#   make test-e2e-(ci/local) CREATEDELETE=y   — Create Delete Volume
#   make test-e2e-(ci/local) NETPOL=y         — NetworkPolicy Enforcement
#   make test-e2e-(ci/local) WEBHOOK=y        — Snapshot Validation Webhook
#   make test-e2e-(ci/local) WEBHOOK_CM=y     — Snapshot Validation Webhook (cert-manager)
GINKGO_FOCUS = $(if $(NETPOL),--ginkgo.focus="NetworkPolicy Enforcement",) \
               $(if $(WEBHOOK),--ginkgo.focus="Snapshot Validation Webhook",) \
               $(if $(WEBHOOK_CM),--ginkgo.focus="Snapshot Validation Webhook (cert-manager)",) \
               $(if $(CREATEDELETE),--ginkgo.focus="Create Delete Volume",) \
               $(if $(LIST),--ginkgo.focus="List Volumes",) \
               $(if $(RESIZE_UNATTACHED),--ginkgo.focus="Resize Volume Unattached",) \
               $(if $(RESIZE_EXT4),--ginkgo.focus="Resize Volume$$",) \
               $(if $(RESIZE_XFS),--ginkgo.focus="Resize Volume XFS",) \
               $(if $(RESIZE),--ginkgo.focus="Resize Volume",) \
               $(if $(PERSISTENCE),--ginkgo.focus="Attach Detach Volume",) \
               $(if $(SNAPSHOT),--ginkgo.focus="Create Snapshot And Restore",)

# CI-friendly e2e test — used in matrix strategy where each job runs one test.
# Use named shortcuts to run a single test case.
.PHONY: test-e2e-ci
test-e2e-ci:
	@echo "==> Running e2e tests"
	cd test/e2e && go test -tags e2e -v -timeout 30m $(GINKGO_FOCUS) ./...

# Local-development variant — deploys with netpol, then runs tests.
# Use named shortcuts to run a single test case.
.PHONY: test-e2e-local
test-e2e-local: deploy-test
	@echo "==> Running e2e tests (verbose mode)"
	cd test/e2e && go test -tags e2e -v --ginkgo.output-interceptor-mode=none -timeout 30m $(GINKGO_FOCUS) ./...

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
		kubeconform --ignore-missing-schemas /tmp/upcloud-csi-rendered.yaml; \
	else \
		echo "kubeconform not installed. Install from https://github.com/yannh/kubeconform"; \
		exit 1; \
	fi
