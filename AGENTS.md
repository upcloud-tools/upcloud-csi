# Agent preferences

## GitHub Actions

- Pin to a specific release like `ubuntu-24.04`. Never use `ubuntu-latest`.
- Pin every action by commit SHA with a comment containing the readable version, e.g. `actions/checkout@df4cb1c0... # v6.0.3`.
- Do NOT add install steps for tools that ship with the base image (`kubectl`, `docker`, `git`, `curl`, etc.). Check the [current image contents](https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md) before adding an install step.
- Container builds use `buildah` via `redhat-actions/buildah-build` + `redhat-actions/push-to-registry`. Containerfile at `cmd/upcloud-csi-plugin/Containerfile`.

## Versioning and changelogs

- Two separate changelogs, never mix: `/CHANGELOG.md` for app (Go code), `deploy/helm/CHANGELOG.md` for Helm chart (templates, values, schema)
- App version format `vX.Y.Z` — tracked in `deploy/helm/Chart.yaml` (`appVersion`) and root `CHANGELOG.md`
- Chart version format `X.Y.Z` — tracked in `deploy/helm/Chart.yaml` (`version`) and Helm `CHANGELOG.md`
- When Go code changes: bump `appVersion`, update root `CHANGELOG.md`
- When Helm template/values change: bump chart `version`, update Helm `CHANGELOG.md`
- `artifacthub.io/changes` in `Chart.yaml`: replace with the changes for the current version only, not cumulative

## Go

- Version: `1.26` (must match `go.mod` and Containerfile)
- Test: `make test` runs `go vet ./... && go test -race ./...`
- Lint: `golangci-lint` with config in `.golangci.yml`. Run via `cd test/e2e && golangci-lint run --timeout=2m ./testruns/` or pre-commit.

## Helm chart

- `# @schema` annotations inline on same line as value: `fieldName: value  # @schema type:[integer, null]`
- Token-based auth is primary for new deployments (`credentials.token` + `credentials.tokenKey` with `createSecret: true`). Existing `upcloud` secrets with `username`/`password` still work — all three env vars are `optional: true` for backward compat.
- Snapshot validation webhook: optional cert-manager TLS via `snapshotValidationWebhook.certManager` block (requires an existing Issuer or ClusterIssuer).
- Tests: `make helm-unittest`. Lint: `make helm-lint`
- Static analysis: `make kube-lint` (kube-linter) and `make k8s-lint` (kubeconform), both gated in pre-commit on `files: ^deploy/helm/`

## Testing

- `make test` — unit tests for packages under `./internal/...` and `./cmd/...`
- `make helm-unittest` — Helm chart unit tests
- `make test-e2e-local LIST=y` — single e2e deploy, then run one test
- `make test-e2e-local SNAPSHOT=y PERSISTENCE=y` — multiple e2e tests in one run
- E2e shortcut flags: `SNAPSHOT`, `RESIZE`, `RESIZE_EXT4`, `RESIZE_XFS`, `LIST`, `CREATEDELETE`, `PERSISTENCE`, `NETPOL`, `WEBHOOK`
- The e2e deploy step (`deploy-test`) hardcodes `--set networkPolicy.enabled=true` and accepts `HELM_OPTS` for additional `--set` flags
- For local e2e runs: `export KUBECONFIG=~/.kube/gh-csi-test-cluster_kubeconfig.yaml`
- `make test-e2e-ci ${{ matrix.test-case }}=y` — CI e2e (no deploy, assumes already deployed)
- Code scanning: CodeQL (`codeql-analysis.yaml`) + Trivy (`trivy-scan.yaml`) in CI. Trivy CVEs for indirect tooling deps suppressed via `.trivyignore.yaml` at repo root.

## Release

- `release.yaml` (app): automatic on push to `main` when `deploy/helm/Chart.yaml`'s `appVersion` is bumped. Detects the bump, builds the SHA image, retags to version tag + latest via `oras copy` (server-side, zero transfer), signs with cosign (keyless), creates a draft GitHub release with release notes, and auto-creates the git tag. No manual tagging needed.
- `release-helm.yaml`: automatic on push to `main` when `deploy/helm/Chart.yaml` changes. Packages, pushes to OCI, and creates a `helm-v<version>` GitHub release.
- `workflow_dispatch`: available on both workflows as a manual override.

## Pre-commit

Configured in `.pre-commit-config.yaml`:

| Hook | Runs | Triggers on |
|------|------|-------------|
| `golangci-lint` | linter | `.go` files (upstream config) |
| `test` | `make test` | `.go` files |
| `kube-lint` | `make kube-lint` | `deploy/helm/` files |
| `k8s-lint` | `make k8s-lint` | `deploy/helm/` files |
