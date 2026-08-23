TAG ?= $(shell git describe --tags)
COMMIT = $(shell git log --format="%h" -n 1)
TREE_STATE = $(shell git diff --quiet && echo 'clean' || echo 'dirty')
TARGETARCH ?= amd64

CONTAINER_REPO ?= ghcr.io/upcloud-tools/upcloud-csi-test
IMAGE_TAG ?= $(shell git rev-parse HEAD)


# Build container image
# TAG: version string
# COMMIT: short commit hash
# TREE_STATE: "clean" or "dirty" based on git status
# CONTAINER_REPO: full image name including registry
# IMAGE_TAG: tag to apply to the built image
.PHONY: container-build
container-build:
	TARGETARCH=$(TARGETARCH) VERSION=$(TAG) COMMIT=$(COMMIT) TREE_STATE=$(TREE_STATE) \
	IMAGE=$(CONTAINER_REPO) IMAGE_TAG=$(IMAGE_TAG) ./build-image.sh

# Push container image to registry, optionally capturing digest
# CONTAINER_REPO: full image name including registry
# IMAGE_TAG: tag of the image to push
# DIGEST_FILE: optional path where the image digest will be written
.PHONY: push-image
push-image:
	@echo "==> Pushing image $(CONTAINER_REPO):$(IMAGE_TAG)"
ifdef DIGEST_FILE
	buildah push --digestfile "$(DIGEST_FILE)" "$(CONTAINER_REPO):$(IMAGE_TAG)" "docker://$(CONTAINER_REPO):$(IMAGE_TAG)"
else
	buildah push "$(CONTAINER_REPO):$(IMAGE_TAG)"
endif

# Install or upgrade the driver via Helm into kube-system
# HELM_VALUES: optional values file(s)
# HELM_OPTS: extra helm flags
.PHONY: helm-deploy
helm-deploy:
	helm upgrade --install upcloud-csi $(HELM_CHART_DIR) --namespace kube-system \
		$(if $(HELM_VALUES),--values $(HELM_VALUES),) \
		$(HELM_OPTS) --wait --timeout 180s

CERT_MANAGER_VERSION ?= v1.20.3

# Install cert-manager and wait for it to become ready
.PHONY: install-cert-manager
install-cert-manager:
	helm repo add jetstack https://charts.jetstack.io
	helm upgrade --install cert-manager jetstack/cert-manager \
		--namespace cert-manager --create-namespace \
		--version $(CERT_MANAGER_VERSION) \
		--set crds.enabled=true
	kubectl wait --namespace cert-manager --for=condition=Available deployment cert-manager --timeout=180s
	kubectl wait --namespace cert-manager --for=condition=Available deployment cert-manager-webhook --timeout=180s

# Apply the self-signed ClusterIssuer used by e2e tests
.PHONY: create-e2e-clusterissuer
create-e2e-clusterissuer:
	kubectl apply -f test/e2e/clusterissuer.yaml

# Deploy the driver for e2e testing: cert-manager, ClusterIssuer, then the chart
# with NetworkPolicy enforcement enabled and the locally built image.
# CONTAINER_REPO: image repository under test
# IMAGE_TAG: image tag to deploy
.PHONY: deploy-test
deploy-test: install-cert-manager create-e2e-clusterissuer
	helm upgrade --install upcloud-csi $(HELM_CHART_DIR) --namespace kube-system \
		--set networkPolicy.enabled=true \
		--set clusterZone=de-fra1 \
		--set image.repository=$(CONTAINER_REPO) \
		--set image.tag=$(IMAGE_TAG) \
		--set image.pullPolicy=Always \
		$(if $(HELM_VALUES),--values $(HELM_VALUES),) \
		$(HELM_OPTS) --wait --timeout 180s

# Apply the e2e test storage classes
.PHONY: deploy-test-sc
deploy-test-sc:
	kubectl apply -f test/e2e/test-storage-classes.yaml

# Remove leftovers from an e2e run: test namespace, snapshot contents and classes.
# NAMESPACE: namespace to delete
# TEST_RUN_ID: csi-test label of the run to clean up
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

# Run Go unit tests (vet + race detector) for all packages
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

# Run each fuzz target for 30 seconds
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

# Run integration tests from test/integration (requires UPCLOUD_TEST_* env vars)
.PHONY: test-integration
test-integration:
	make -C test/integration test


# Named shortcuts for running e2e tests locally or in CI:
#   make test-e2e-(ci/local) SNAPSHOT=y          — Create Snapshot And Restore
#   make test-e2e-(ci/local) RESIZE=y            — Resize Volume (ext4 + xfs, sequential)
#   make test-e2e-(ci/local) RESIZE_EXT4=y       — Resize Volume (ext4 only)
#   make test-e2e-(ci/local) RESIZE_XFS=y        — Resize Volume (xfs only)
#   make test-e2e-(ci/local) RESIZE_UNATTACHED=y — Resize Unattached Volume
#   make test-e2e-(ci/local) LIST=y              — List Volumes
#   make test-e2e-(ci/local) PERSISTENCE=y       — Attach Detach Volume
#   make test-e2e-(ci/local) CREATEDELETE=y      — Create Delete Volume
#   make test-e2e-(ci/local) NETPOL=y            — NetworkPolicy Enforcement
#   make test-e2e-(ci/local) WEBHOOK=y           — Snapshot Validation Webhook
#   make test-e2e-(ci/local) WEBHOOK_CM=y        — Snapshot Validation Webhook (cert-manager)
#   make test-e2e-(ci/local) FILESTORAGE=y       — File Storage Dynamic Provisioning
#   make test-e2e-(ci/local) FS_EXPAND=y         — File Storage Expand
#   make test-e2e-(ci/local) FS_CONCURRENT=y     — File Storage Concurrent RWX
GINKGO_FOCUS = $(if $(NETPOL),--ginkgo.focus="NetworkPolicy Enforcement",) \
               $(if $(WEBHOOK),--ginkgo.focus="Snapshot Validation Webhook",) \
               $(if $(WEBHOOK_CM),--ginkgo.focus="Snapshot Validation Webhook (cert-manager)",) \
               $(if $(CREATEDELETE),--ginkgo.focus="Create Delete Volume",) \
               $(if $(LIST),--ginkgo.focus="List Volumes",) \
               $(if $(RESIZE_UNATTACHED),--ginkgo.focus="Resize Volume Unattached",) \
               $(if $(RESIZE_EXT4),--ginkgo.focus="Resize Volume$$",) \
               $(if $(RESIZE_XFS),--ginkgo.focus="Resize Volume XFS",) \
               $(if $(RESIZE),--ginkgo.focus="Resize Volume",) \
               $(if $(FILESTORAGE),--ginkgo.focus="File Storage Dynamic Provisioning",) \
               $(if $(FS_EXPAND),--ginkgo.focus="File Storage Expand",) \
               $(if $(FS_CONCURRENT),--ginkgo.focus="File Storage Concurrent RWX",) \
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

# Extract release notes for the current version from CHANGELOG.md
# TAG: version to extract notes for
.PHONY: release-notes
release-notes: CHANGELOG_HEADER = ^\#\# \[
release-notes: CHANGELOG_VERSION = $(subst v,,$(TAG))
release-notes:
	@awk \
		'/${CHANGELOG_HEADER}${CHANGELOG_VERSION}/ { flag = 1; next } \
		/${CHANGELOG_HEADER}/ { if ( flag ) { exit; } } \
		flag { if ( n ) { print prev; } n++; prev = $$0 }' \
		CHANGELOG.md

# Extract release notes for the current chart version from CHANGELOG.md
# HELM_CHART_VERSION: auto-detected from Chart.yaml version field
HELM_CHART_VERSION = $(or $(CHART_VERSION),$(shell awk '/^version:/ {print $$2}' deploy/helm/Chart.yaml))
.PHONY: helm-release-notes
helm-release-notes:
	@awk \
		'/^## \['$(HELM_CHART_VERSION)'\]/ { flag = 1; next } \
		/^## \[/ { if ( flag ) { exit; } } \
		flag { if ( n ) { print prev; } n++; prev = $$0 }' \
		$(HELM_CHART_DIR)/CHANGELOG.md

HELM_CHART_DIR = deploy/helm

# Run Helm chart unit tests (installs the unittest plugin if missing)
.PHONY: helm-unittest
helm-unittest:
	helm plugin install https://github.com/helm-unittest/helm-unittest.git 2>/dev/null || true
	helm unittest $(HELM_CHART_DIR)

# Lint the Helm chart
.PHONY: helm-lint
helm-lint:
	helm lint $(HELM_CHART_DIR)

# Run all Helm tests (lint + unit tests)
.PHONY: helm-test
helm-test: helm-lint helm-unittest

# Package the Helm chart into dist/
.PHONY: helm-package
helm-package:
	mkdir -p dist
	helm package $(HELM_CHART_DIR) --destination dist

# Lint Kubernetes manifests with kube-linter
.PHONY: kube-lint
kube-lint:
	kube-linter lint --config $(HELM_CHART_DIR)/.kube-linter.yaml $(HELM_CHART_DIR)

# Render the chart with helm template and validate the manifests with kubeconform
.PHONY: k8s-lint
k8s-lint:
	helm template test-release $(HELM_CHART_DIR) > /tmp/upcloud-csi-rendered.yaml
	@if command -v kubeconform > /dev/null 2>&1; then \
		kubeconform --ignore-missing-schemas /tmp/upcloud-csi-rendered.yaml; \
	else \
		echo "kubeconform not installed. Install from https://github.com/yannh/kubeconform"; \
		exit 1; \
	fi
