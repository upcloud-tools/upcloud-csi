# Releasing

Both app and Helm chart releases are triggered automatically by pushing to `main`.

### App release

Bumping `appVersion` in `deploy/helm/Chart.yaml` triggers the **Build and Release** workflow:

1. Bump `appVersion` in `deploy/helm/Chart.yaml`
2. Add the release entry to root `CHANGELOG.md`
3. Push/merge to `main`

The workflow automatically:
- Builds the container image (commit SHA tag)
- Copies it to the version tag and `latest` via `oras copy` (server-side, zero transfer)
- Signs the image with cosign keyless signing
- Extracts release notes from `CHANGELOG.md`
- Creates a **draft** GitHub release
- Tags the commit and pushes the tag

4. Verify the [release notes](https://github.com/upcloud-tools/upcloud-csi/releases)
5. Publish the draft

### Helm chart release

Bumping `version` in `deploy/helm/Chart.yaml` triggers the **Release Helm chart** workflow:

1. Update `version` in `deploy/helm/Chart.yaml`
2. Update `deploy/helm/CHANGELOG.md` and the `artifacthub.io/changes` annotation
3. Push/merge to `main`

The workflow automatically packages the chart, pushes it to the OCI registry, pushes Artifact Hub metadata, and creates a `helm-v<version>` GitHub release.
