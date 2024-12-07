# Release

The Prometheus Go client library follows a release process similar to the [Prometheus server](https://github.com/prometheus/prometheus/blob/main/RELEASE.md).

## Branch Management

We use [Semantic Versioning](https://semver.org/).

- Maintain separate `release-<major>.<minor>` branches
- Branch protection enabled automatically for `release-*` branches
- Bug fixes go to latest release branch, then merge to main
- Features and changes go to main branch
- Older release branches maintained on best-effort basis

## Pre-Release Preparations

1. Review main branch state:
   - Expedite critical bug fixes
   - Hold back risky changes
   - Update dependencies via Dependabot PRs
   - Check for security alerts

## Cutting a Minor Release

1. Create release branch:

   ```bash
   git checkout -b release-<major>.<minor> main
   git push origin release-<major>.<minor>
   ```

2. Create feature branch:

   ```bash
   git checkout -b <yourname>/cut-<major>.<minor>.0 release-<major>.<minor>
   ```

3. Update version and documentation:
   - Update `VERSION` file
   - Update `CHANGELOG.md` (user-impacting changes)
   - Order: [SECURITY], [CHANGE], [FEATURE], [ENHANCEMENT], [BUGFIX]
   - For RCs, append `-rc.0`

4. Create PR and get review

5. After merge, create tags:

   ```bash
   tag="v$(< VERSION)"
   git tag -s "${tag}" -m "${tag}"
   git push origin "${tag}"
   ```

6. For Release Candidates:
   - Create PR against prometheus/prometheus using RC version
   - Create PR against kubernetes/kubernetes using RC version
   - Make sure the CI is green for the PRs
   - Allow 1-2 days for downstream testing
   - Fix any issues found before final release
   - Use `-rc.1`, `-rc.2` etc. for additional fixes

7. For Final Release:
   - Wait for CI completion
   - Verify artifacts published
   - Click "Publish release"
   - For RCs, ensure "pre-release" box is checked

8. Announce release:
   - <prometheus-announce@googlegroups.com>
   - Slack
   - x.com/BlueSky

9. Merge release branch to main:

   ```bash
   git checkout main
   git merge --no-ff release-<major>.<minor>
   ```

## Cutting a Patch Release

1. Create branch from release branch:

   ```bash
   git checkout -b <yourname>/cut-<major>.<minor>.<patch> release-<major>.<minor>
   ```

2. Apply fixes:
   - Cherry-pick from main: `git cherry-pick <commit>`
   - Or add new fix commits

3. Follow steps 3-9 from minor release process

## Handling Merge Conflicts

If conflicts occur merging to main:

1. Create branch: `<yourname>/resolve-conflicts`
2. Fix conflicts there
3. PR into main
4. Leave release branch unchanged

## Note on Versioning

Go modules require strict semver. Because we don't commit to avoid breaking changes between minor releases, we use major version zero releases for libraries.

## Compatibility Guarantees

### Supported Go Versions

- Support provided only for the three most recent major Go releases
- While the library may work with older versions, no fixes or support provided
- Each release documents the minimum required Go version

### API Stability

The Prometheus Go client library aims to maintain backward compatibility within minor versions, similar to [Go 1 compatibility promises](https://golang.org/doc/go1compat). However, as indicated by the major version zero (v0):

- API signatures may change between minor versions
- Types may be modified or relocated
- Default behaviors might be altered
- Feature removal/deprecation can occur with minor version bump

### Compatibility Testing

Before each release:

1. **Internal Testing**:
   - Full test suite must pass
   - Integration tests with latest Prometheus server
   - Benchmark comparisons with previous version

2. **External Validation**:
   - Testing with prometheus/prometheus master branch
   - Testing with kubernetes/kubernetes master branch
   - Breaking changes must be documented in CHANGELOG.md

### Version Policy

- Bug fixes increment patch version (e.g., v0.9.1)
- New features increment minor version (e.g., v0.10.0)
- Breaking changes increment minor version with clear documentation
- Major version remains at 0 to indicate potential instability

### Deprecation Policy

1. Features may be deprecated in any minor release
2. Deprecated features:
   - Will be documented in CHANGELOG.md
   - Will emit warnings when used (when possible)
   - May be removed in next minor version
   - Must have migration path documented
