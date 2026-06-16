# Releasing

1. Merge all changes to `main`
2. Update CHANGELOG.md
   - Rename `## [Unreleased]` to the new version, e.g. `## [2.1.0] - 2026-06-16`
   - Add a new empty `## [Unreleased]` section above it
   - Update the version compare links at the bottom of the page
3. Commit the changelog update
4. Tag the commit with the version, e.g. `v2.1.0`
   ```bash
   git tag v2.1.0
   ```
5. Push the tag to GitHub
   ```bash
   git push origin v2.1.0
   ```
   This triggers two GitHub Actions workflows:
   - **Release** — creates a draft GitHub Release with changelog as release notes
   - **Build and push container image** — builds the multistage container image and pushes to GHCR
6. Verify the [release notes](https://github.com/upcloud-tools/upcloud-csi/releases)
7. Publish the drafted release
