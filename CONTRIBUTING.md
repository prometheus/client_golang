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

**Example from our codebase:**
- [`prometheus/common`](https://github.com/prometheus/common) has many dependencies, including
  [`kingpin`](https://github.com/alecthomas/kingpin) for CLI parsing (used only in
  [`promslog/flag`](https://github.com/prometheus/common/tree/main/promslog/flag)).
- [`client_golang`](https://github.com/prometheus/client_golang) depends on `common`.
- But `client_golang` users don't get `kingpin` as an indirect dependency because `client_golang`
  doesn't import `promslog/flag` anywhere.

**Testing this:**

To verify a dependency doesn't propagate to users, create a test module:

```bash
# 1. Create a test module that imports client_golang
mkdir /tmp/test-client-golang-deps && cd /tmp/test-client-golang-deps
go mod init example.com/test
go get github.com/prometheus/client_golang

# 2. Check if the dependency appears in your module
go mod why github.com/alecthomas/kingpin
# Expected output: (main module does not need package github.com/alecthomas/kingpin)

# 3. Verify it's not in go.mod
grep kingpin go.mod
# Expected: no output (kingpin is not propagated)
```

To check why a dependency IS in `client_golang`'s own go.mod (run from this repository):

```bash
# See why client_golang needs a package
go mod why github.com/prometheus/common
# Output shows the import chain from client_golang code

# See full dependency graph
go mod graph | grep common
```

### Adding New Dependencies

**Key Concerns:**

When evaluating new dependencies, critical factors include:

- **Security:** Vulnerabilities in our dependency chain affect all users
- **Transitive dependencies:** How many indirect dependencies does it bring?
- **Version conflicts:** Can impact downstream projects (like KEDA, Kubernetes, and other users)
- **Licensing:** Must be Apache 2.0 compatible (avoid GPL, LGPL, or copyleft licenses)

**Process for adding production code dependencies:**

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

### Resources

- [Go Modules Reference](https://go.dev/ref/mod) - comprehensive but dense
- [Semantic Versioning](https://semver.org/)
- [Go Security Best Practices](https://go.dev/security/best-practices)
