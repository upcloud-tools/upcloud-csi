# Agent preferences

- **Ubuntu image**: Pin to a specific release like `ubuntu-24.04`. Never use `ubuntu-latest` because it silently shifts underfoot — when a newer release comes out (e.g. `ubuntu-26.04`), I want to deliberately choose when to upgrade rather than have it happen automatically.
- **GitHub Actions**: Pin every action by commit SHA with a comment containing the readable version, e.g. `actions/checkout@df4cb1c0... # v6.0.3`
- **Go version**: `1.26`, set via `GO_VERSION` workflow-level env var, referenced as `${{ env.GO_VERSION }}` in `setup-go` steps
- **Pre-installed tools on github runners**: Do NOT add install steps for tools that ship with the base image (`kubectl`, `docker`, `git`, `curl`, etc.). Only install extra actions when they provide meaningful benefits like caching (e.g. `setup-go`). Check the [current image contents](https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md) before adding an install step.
- **Container builds**: Use `buildah` for building and pushing container images. The file should be named `Containerfile`. In GitHub Actions, use `redhat-actions/buildah-build` and `redhat-actions/push-to-registry`.
- **Security policy**: Defined in `SECURITY.md` at the repo root. Direct reporters to GitHub's Private vulnerability reporting tool.
- **Code scanning**: Use `github/codeql-action` (latest v3.x, pinned by SHA) with `go` language matrix.
- **Dependabot**: Config in `.github/dependabot.yml` tracks `gomod`, `github-actions`, and `docker` ecosystems daily.
- **Versioning and changelogs**:
  - Two separate changelogs, never mix: `/CHANGELOG.md` for app (Go code), `deploy/helm/CHANGELOG.md` for Helm chart (templates, values, schema)
  - App version format `vX.Y.Z` — tracked in `deploy/helm/Chart.yaml` (`appVersion`) and root `CHANGELOG.md`
  - Chart version format `X.Y.Z` — tracked in `deploy/helm/Chart.yaml` (`version`) and Helm `CHANGELOG.md`
  - When Go code changes: bump `appVersion`, update root `CHANGELOG.md`
  - When Helm template/values change: bump chart `version`, update Helm `CHANGELOG.md`
  - `artifacthub.io/changes` in `Chart.yaml`: replace with the changes for the current version only, not cumulative across versions
- **Helm chart conventions**:
  - `# @schema` annotations inline on same line as value (Traefik style): `fieldName: value  # @schema type:[integer, null]`
  - Run helm unit tests via `make helm-unittest`
