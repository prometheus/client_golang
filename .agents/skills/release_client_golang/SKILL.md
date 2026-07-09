---
name: release_client_golang
description: How to release prometheus/client_golang and verify the release process.
---

# Releasing prometheus/client_golang

This skill guides you through the process of releasing `prometheus/client_golang` and evaluating if the release steps are followed correctly.

The release process involves parallel tracks: cutting the release branch and starting compatibility testing in a subagent, while preparing the changelog PR in the main agent.

## Prerequisites

- Access to `prometheus/client_golang` repository.
- `git` installed and configured.
- `gh` (GitHub CLI) installed and authenticated (optional but recommended for PRs and releases).
- Go environment installed.

## Release Types

Determine if you are cutting a **Minor** or a **Patch** release.

- **Minor Release**: Increments the minor version (e.g., v1.21.0). Includes new features and changes. Starts from `main` branch.
- **Patch Release**: Increments the patch version (e.g., v1.21.1). Includes only bug fixes. Starts from the corresponding `release-<major>.<minor>` branch.

---

## Phase 1: Initiation & Parallel Verification

### 1. Cut the Release Branch (For Minor Release)
If this is a minor release, start by creating the release branch.

```bash
git checkout main
git pull origin main
git checkout -b release-<major>.<minor>
git push origin release-<major>.<minor>
```

*(For Patch releases, the `release-<major>.<minor>` branch already exists. Ensure you have the latest local copy).*

### 2. Delegate Compatibility Testing to Subagent
Once the release branch exists (or is identified for patch), delegate the verification to a subagent so it can run in parallel while you prepare the changelog.

**Instruction to main agent:**
Invoke a subagent (e.g., using `self` or a runner agent) with the following prompt:
> Run compatibility testing for `prometheus/client_golang` release branch `release-<major>.<minor>`.
> Follow these steps:
> 1. Run internal tests: `go test -v ./...`
> 2. Run benchmarks (optional): `go test -count=6 -benchmem -bench .`
> 3. Test downstream with Prometheus: Clone `prometheus/prometheus`, update `go.mod` to use this release branch, and run tests.
> 4. Test downstream with Kubernetes: Clone `kubernetes/kubernetes`, pin dependency to this branch, and verify vendor.
> Report back with results.

### Evaluation/Verification of Phase 1 Initiation
- Is the `release-<major>.<minor>` branch created and pushed?
- Has the subagent been successfully invoked to run compatibility tests?

---

## Phase 2: Preparing the Release PR (Parallel to Phase 1)

While the subagent is verifying compatibility, prepare the release PR.

### 1. Create Work Branch
On top of the release branch, create your work branch:
```bash
git checkout release-<major>.<minor>
git pull origin release-<major>.<minor>
git checkout -b <yourname>/cut-<major>.<minor>.<patch-or-rc>
```

### 2. Update Version and Changelog
- Update `VERSION` file (e.g., `1.21.0-rc.0` for RC, or `1.21.1` for patch).
- Update `CHANGELOG.md`. Group changes by:
  - `[SECURITY]`
  - `[CHANGE]`
  - `[FEATURE]`
  - `[ENHANCEMENT]`
  - `[BUGFIX]`
- Document the minimum required Go version.

### 3. Submit PR
- Push branch and create PR against `release-<major>.<minor>` branch.
- Request review.

### Evaluation/Verification of Phase 2
- Is `VERSION` file correctly formatted?
- Is `CHANGELOG.md` updated with correct headers and sorted?
- Is the PR targeted to the correct `release-*` branch (NOT `main`)?

---

## Phase 3: Tagging and Publishing

This phase requires both Phase 1 (Compatibility Testing) and Phase 2 (Release PR) to be completed.

1. **Verify Prerequisites**:
   - Ensure the subagent reported that all compatibility tests passed.
   - Ensure the release PR is approved and merged into `release-<major>.<minor>`.

2. **Tag the Release**:
   ```bash
   git checkout release-<major>.<minor>
   git pull origin release-<major>.<minor>
   tag="v$(cat VERSION)"
   git tag -s "${tag}" -m "${tag}"
   git push origin "${tag}"
   ```

3. **Publish on GitHub**:
   - Create a draft release on GitHub matching the tag.
   - Copy-paste the relevant `CHANGELOG.md` section.
   - For RCs: Check "This is a pre-release" box.
   - For Final: Check "Set as the latest release" (if it is the latest minor).
   - Publish.

4. **Downstream PRs (For RCs)**:
   - Create PR to `prometheus/prometheus` updating `client_golang` to the RC.
   - Create PR to `kubernetes/kubernetes` updating `client_golang` to the RC.
   - Monitor CI on these PRs.

### Evaluation/Verification of Phase 3
- Does the git tag match the `VERSION` file?
- Is the tag signed (`git tag -s`)?
- Is the GitHub release description matching the changelog?
- Are downstream PRs created and CI passing?

---

## Phase 4: Post-Release

1. **Merge release branch back to main**:
   ```bash
   git checkout main
   git pull origin main
   git merge --no-ff release-<major>.<minor>
   # Resolve conflicts if any (using a separate branch if needed)
   git push origin main
   ```

2. **Announce**:
   - Email `prometheus-announce@googlegroups.com`
   - Announce on Slack and Social Media.

### Evaluation/Verification of Phase 4
- Is `main` branch updated with the release changes?
- Was the merge done with `--no-ff`?
