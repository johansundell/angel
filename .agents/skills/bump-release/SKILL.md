---
name: bump-release
description: Bump version number, run tests, commit and push directly to main, and run make release. Use when asked to bump version, cut a release, publish a release, or /bump-release.
---

# Bump and Release

Workflow for bumping the application version, committing directly to `main`, and publishing a new release via `make release`.

## Prerequisites

1. Working directory must be on `main` branch.
2. Working tree must be clean (no uncommitted or untracked changes).
3. Local `main` must be up to date with `origin/main`.
4. Release tools installed (`make deps` if `github-release` or `gox` are missing).
5. GitHub credentials available for `github-release` (e.g., `GITHUB_TOKEN` environment variable).

## Quick Execution

The release workflow is automated via [`scripts/bump-release.sh`](../../scripts/bump-release.sh) and the `make bump-release` target:

```bash
# Default: bump patch version (e.g. v0.0.13 -> v0.0.14)
make bump-release

# Specify bump level or explicit version:
make bump-release BUMP=minor
make bump-release BUMP=major
make bump-release BUMP=v0.1.0

# Dry-run verification (verifies tests and prints planned actions without pushing or releasing):
make bump-release DRY_RUN=1
```

## Step-by-Step Procedure

If executing manually or troubleshooting, follow these steps in order:

1. **Verify branch and working tree**:
   ```bash
   git checkout main
   git pull origin main
   git status --porcelain
   ```
   *Completion criterion*: Output of `git status --porcelain` is empty, and branch is `main`.

2. **Determine target version**:
   - Inspect `VERSION` in [`Makefile`](../../Makefile) (e.g. `VERSION := v0.0.13`).
   - Increment the patch segment by default (or minor/major as requested).

3. **Update Makefile**:
   - Edit `VERSION := <new-version>` in `Makefile`.

4. **Verify tests**:
   ```bash
   go test ./...
   ```
   *Completion criterion*: All tests pass. If any test fails, revert `Makefile` and abort.

5. **Commit and push directly to main**:
   ```bash
   git add Makefile
   git commit -m "chore: bump version to <new-version>"
   git push origin main
   ```
   *Completion criterion*: Commit is pushed to `origin/main`. Note: `AGENTS.md` explicitly permits direct commits to `main` for release version bumps.

6. **Execute release**:
   ```bash
   make release
   ```
   *Completion criterion*: `make release` builds cross-platform binaries, generates checksums, publishes the GitHub Release with notes via `github-release`, and pulls the created release tag.

7. **Verify deployment**:
   - Check GitHub release: `gh release view <new-version>`
   - The release event triggers GitHub Actions (`docker.yml` and `deploy.yml`) to build Docker images and deploy to VPS.
