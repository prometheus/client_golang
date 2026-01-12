# Contributing

Thank you for contributing to our project! Here are the steps and guidelines to follow when creating a pull request (PR).

Prometheus uses GitHub to manage reviews of pull requests.

* If you have a trivial fix or improvement, go ahead and create a pull request,
  addressing (with `@...`) the maintainer of this repository (see
  [MAINTAINERS.md](MAINTAINERS.md)) in the description of the pull request.

* If you plan to do something more involved, first discuss your ideas
  on our [mailing list](https://groups.google.com/forum/?fromgroups#!forum/prometheus-developers).
  This will avoid unnecessary work and surely give you and us a good deal
  of inspiration.

* Relevant coding style guidelines are the [Go Code Review
  Comments](https://code.google.com/p/go-wiki/wiki/CodeReviewComments)
  and the _Formatting and style_ section of Peter Bourgon's [Go: Best
  Practices for Production
  Environments](http://peter.bourgon.org/go-in-production/#formatting-and-style).

* Be sure to sign off on the [DCO](https://github.com/probot/dco#how-it-works)

## Managing Dependencies

`client_golang` is a critical library in the Prometheus ecosystem, used by thousands of projects
including Kubernetes. Any dependency we add becomes a transitive dependency for all our users,
which is why we must be extremely careful when adding or updating dependencies.

### Why We're Strict About Dependencies

**The Kubernetes Factor:**
Kubernetes depends on `client_golang`, and adding dependencies creates significant work for the
Kubernetes maintainers and affects the entire Kubernetes ecosystem. This dependency chain means
our decisions impact far more projects than just our direct users.

**The Transitive Dependency Problem:**
Every dependency we add potentially becomes a dependency for thousands of projects, bringing with it:
- Increased attack surface for security vulnerabilities in our dependency chain
- Potential version conflicts with users' other dependencies

### Understanding Indirect Dependencies

**Key Insight:** Not all indirect dependencies propagate to our users!

Go modules are smarter than you might think. An indirect dependency in our `go.mod` doesn't
automatically become an indirect dependency for projects that import `client_golang`.

**How it works:**
- If we depend on package A, and A depends on package B, but we never import/use anything from B,
  then B will **not** propagate to projects that import `client_golang`.
- Go modules only propagate dependencies that are actually used in the import chain

**Example from our codebase:**
- `prometheus/common` has many dependencies (e.g., `kingpin` for CLI parsing)
- `client_golang` depends on `common`
- But `client_golang` users don't get `kingpin` as an indirect dependency because `client_golang`
  doesn't import the parts of `common` that use `kingpin`

**Testing this:**
```bash
# See why a package is in our dependencies
go mod why github.com/some/package

# See the full dependency graph
go mod graph

# Test if a dependency propagates by creating a test module that imports client_golang
# and checking if the dependency appears in its go.mod
```

**Important exception - Examples can leak dependencies:**
Code in `examples/` or example test files (like `example_test.go`) that imports packages will
cause those dependencies to appear in `go.mod` and potentially leak to users. Testing during
recent dependency discussions showed that example code imports (such as `api/prometheus/v1/example_test.go`
importing `prometheus/common/config`) can cause many indirect dependencies to leak.

### Adding New Dependencies

**Key Concerns:**

When evaluating new dependencies, critical factors include:

- **Security:** Vulnerabilities in our dependency chain affect all users
- **Transitive dependencies:** How many indirect dependencies does it bring?
- **Version conflicts:** Can impact downstream projects (like KEDA, Kubernetes, and other users)
- **Licensing:** Must be Apache 2.0 compatible (avoid GPL, LGPL, or copyleft licenses)

**Process for production code dependencies:**

1. **Open an issue first** to discuss with maintainers
2. **Provide justification:**
   - What problem does it solve?
   - Why can't we implement it ourselves?
   - What alternatives were considered?
3. **Wait for maintainer approval** before implementing
4. **Be prepared for alternatives** if concerns arise during review

**General evaluation criteria to consider:**

- Is the dependency actively maintained?
- Does it have a stable release (prefer v1.0.0+)?
- What is the security track record?
- How complex is its transitive dependency tree?
- Will it benefit the majority of users?

### Vendoring vs. Direct Dependencies

**Prefer direct dependencies** in almost all cases.
Go modules handle version management well, and direct dependencies make it easier to track
updates and security patches.

**Consider vendoring** (via `go mod vendor`) only when:
- We need absolute version stability for a critical dependency
- Upstream is unmaintained but we need the stable code
- There's a specific technical reason (discuss with maintainers first)

**Note:** With Go modules, vendoring is rarely necessary. The `go.sum` file provides
reproducible builds without vendoring.

### Using Unstable Dependencies

**For production code, strongly prefer stable dependencies:**

- Prefer semantic versioned releases (v1.0.0+) when available
- Prefer tagged versions over commit hashes
- Avoid alpha/beta releases when stable alternatives exist

**Pragmatic reality:**

We do use some pre-1.0 dependencies (like `prometheus/client_model v0.6.2`) and have
indirect dependencies with commit-based versions where necessary. The key is:
- New direct dependencies should use stable versions when possible
- Pre-1.0 or commit-based versions require justification
- Discuss with maintainers if unsure

**Example of dependency versioning preferences:**
```go
// Most preferred: Stable semantic version
require github.com/example/lib v1.2.3

// Acceptable if no stable alternative: Pre-1.0 version
require github.com/example/lib v0.6.2

// Least preferred: Commit-based (sometimes unavoidable for indirect deps)
require github.com/example/lib v0.0.0-20230101120000-abcdef123456
```

### Dependencies in Different Contexts

**Production code** (`prometheus/`, `prometheus/...`):
- **Strictest requirements** - affects all users
- Prefer stable versions (v1.0.0+) when available
- Requires maintainer approval
- Minimal transitive dependencies preferred
- Every dependency must be justified

**Test code** (`*_test.go`):
- **More flexible** - test dependencies don't propagate to users!
- Go modules improved in recent versions: test deps stay in our `go.mod` but don't leak to importers
- Can use testing libraries (`go.uber.org/goleak`, `github.com/google/go-cmp`, etc.)
- Still prefer stable, well-maintained tools

**Examples** (`examples/` directory):
- **Most flexible** - example code isn't imported by users
- Can use newer versions to demonstrate features
- Still prefer stable dependencies when possible
- **Warning:** Example imports CAN cause indirect deps to appear in `go.mod`

**Real example discovered in dependency analysis:**

The file `api/prometheus/v1/example_test.go` imports `prometheus/common/config`. Testing
showed that this single import causes many transitive dependencies to appear in our `go.mod`.
Removing such imports would clean up numerous indirect dependencies because those packages
aren't actually used in the production code path.

### Dependency Update Process

When updating dependencies, use standard Go module workflows:

**Checking and updating:**
```bash
# Check for available updates
go list -u -m all

# Update a specific dependency
go get -u github.com/prometheus/client_model@v0.6.2

# Clean up and verify
go mod tidy
go mod verify
```

**Understanding dependency usage:**
```bash
# See why a package is in our dependencies
go mod why github.com/some/package

# See the full dependency graph
go mod graph
```

**Testing:**
```bash
make test
```

**Before committing:**
- Run the full test suite
- Review the dependency's CHANGELOG for breaking changes
- Verify `go.sum` changes are reasonable

**Consider the impact:**
- Will the update affect downstream projects (like Kubernetes)?
- Does it introduce new transitive dependencies?
- Are there any security advisories for the old or new version?

### Red Flags: Dependencies to Reject

When evaluating dependencies, be cautious of:

- Known unpatched security vulnerabilities
- Incompatible licenses (must be Apache 2.0 compatible)
- Excessive transitive dependencies
- Unmaintained projects (no recent activity or releases)
- Poor maintenance indicators (unresponsive maintainers, accumulating issues)

Discuss with maintainers if a dependency raises concerns.

### Example Dependency Evaluation

When proposing a new dependency in an issue, provide analysis covering:

**Basic information:**
- Purpose and which component needs it
- Version and maintenance status
- License compatibility

**Impact analysis:**
- Will it propagate to users? (consider which code imports it)
- How many transitive dependencies does it bring?
- Any security concerns?

**Alternatives:**
- What alternatives were considered?
- Why are they not suitable?

**Recommendation:**
- Your assessment with justification

Maintainers will review and provide feedback before you proceed with implementation.

### Resources

- [Go Modules Reference](https://go.dev/ref/mod) - comprehensive but dense
- [Semantic Versioning](https://semver.org/)
- [Go Security Best Practices](https://go.dev/security/best-practices)

### Key Takeaways

1. **Indirect dependencies don't always propagate** - Go modules are smart about this
2. **Test dependencies don't leak** to projects that import us
3. **Example code can leak dependencies** - be careful with example imports
4. **We're part of the Kubernetes dependency chain** - our choices have wide impact
5. **When in doubt, discuss first** - open an issue before adding dependencies
