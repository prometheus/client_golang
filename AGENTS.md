# Agents Guide for Prometheus client_golang

This document captures patterns, conventions, and preferences observed from maintainer reviews of recently merged pull requests in `prometheus/client_golang` as well as common standards shared across the Prometheus organization. Use this guide to align your contributions with what maintainers expect.

---

## Strict Backwards Compatibility & No Breaking Changes

`client_golang` is a foundational Go library (v1) relied upon by thousands of production services across the Go ecosystem.

- **No Breaking Changes:** Breaking changes to exported APIs, interfaces, struct definitions, or metric registration behaviors in stable packages (`prometheus`, `promhttp`, `api/prometheus/...`) are **strictly prohibited**.
- Do not remove, rename, or alter the signatures of exported functions, methods, or structs.
- Even subtle behavioral breaking changes (such as how labels are validated, how partial matches behave in `DeletePartialMatch`, or how panics are handled across wrapped collectors) must be avoided or safeguarded to preserve existing contracts.
- If an API change or new capability is needed, ensure it is completely backward-compatible or introduce it inside the `exp/` module.

---

## Experimental Features & Decoupling (`exp/` Module)

- **Use the `exp/` Module for Experimentation:** When introducing new capabilities, evolving APIs, unproven performance optimizations (such as sharded per-P counters, TTL eviction mechanisms, or alternative compression techniques), place them in the `exp/` module (`github.com/prometheus/client_golang/exp/...`).
- **Decoupling from Stable v1:** Building inside `exp/` decouples experimental development from the strict compatibility guarantees of the core `prometheus` package. It enables rapid iteration and gathering real-world user feedback without risking breaking changes to stable v1 contracts.
- Note that `exp/` has its own `go.mod` file and dependency management; ensure dependency updates in `exp/` are handled cleanly without impacting the root go module.

---

## API Design & Constructors

- **Avoid Proliferating `<constructor>WithXYZ` Variants:** Avoid introducing new constructor functions following the `<constructor>WithXYZ` or `<constructor>WithOptions` naming patterns (e.g., `NewCollectorWithClientAndTimeout(...)`). This pattern does not scale when new configuration options are introduced over time.
- **Use Config Structs or Functional Options:** Prefer configuration options structs (e.g., `Opts`, `Config`, `HandlerOpts`) passed into standard constructors, functional options, or setting fields where appropriate.
- When adding new parameters or behavior toggles to an existing object, extend its existing options struct (with sensible zero-value defaults) rather than multiplying constructor variants.

---

## Code Quality & Maintainability

- **Avoid Code Duplication:** Keep code DRY (Don't Repeat Yourself) while maintaining clarity and readability. When handling common logic across metric vectors, collector wrappers, or HTTP handlers (`promhttp`), extract clean, reusable internal utilities rather than copy-pasting logic across files.
- **Maintainable Code:** Prioritize long-term maintainability, simplicity, and clean encapsulation over clever or overly complex abstractions. Code should be easy to read, debug, and test for future contributors.
- **Memory & Map Ownership:** Pay strict attention to map and slice aliasing/ownership. When passing `prometheus.Labels` maps or slices into methods (such as `MetricVec.With(labels)` or `GetMetricWith(labels)`), explicitly state and enforce whether the underlying map or slice is copied or retained. Document caller ownership requirements at the interface/method boundary.
- **Collector Concurrency & Safety:** `Collect` and `Gather` operations must be safe for concurrent execution and resilient against unexpected panics (e.g., recovering panics cleanly from wrapped collectors).

---

## PR Title Format

Titles must follow `area: short description`, using a prefix that identifies the subsystem. Examples from merged PRs:

```
api/prometheus: clamp out-of-range formatted timestamps
promhttp: fix grammar in exemplar option doc comments
testutil: add native histogram assertion helpers
fix(prometheus): stabilize label maps in MetricVec.GetMetricWith
chores: remove example Dockerfile and container_description.yaml
build(deps): bump github.com/klauspost/compress from 1.18.7 to 1.19.0 in /exp
```

Common area prefixes in `client_golang`:
- `prometheus`: Core metric primitives, vectors, registry, and collectors.
- `promhttp`: HTTP instrumentation and metrics exposition handlers.
- `api/prometheus`: Client HTTP API packages (`api/prometheus/v1`).
- `exp`: Experimental features, metrics, and sync primitives.
- `testutil`: Test utilities and assertion helpers.
- `tutorials/<name>`: Tutorials and example modules.
- `docs`, `ci`, `build`, `chore`.

For performance-focused pull requests, append `[PERF]` to the area segment or use the `perf(area):` convention.

---

## Commits

- Each commit must compile and pass tests independently across all modules (`.` and `exp/`), except when one commit adds a failing test to expose a bug and the subsequent commit fixes it.
- Keep commits small and focused. Do not bundle unrelated refactoring or formatting changes with bug fixes.
- Sign off every commit with `git commit -s` to satisfy the Developer Certificate of Origin (DCO) requirement.
- Do not include unrelated local modifications or files in your pull request.

---

## Tests

- **Bug Fixes:** Every bug fix requires a regression test that fails without the fix and passes with it.
- **New Behavior / APIs:** New behaviors or exported additions (including inside `exp/`) require comprehensive unit or end-to-end tests.
- **Realistic Mirroring:** Tests should attempt to mirror realistic data structures, concurrency patterns, and usage scenarios.
- **Exported APIs in Tests:** Use only exported APIs in tests (`package prometheus_test`) where possible. This keeps tests closer to real-world library usage and simplifies review.
- **Table-Driven Tests:** Prefer adding test cases to existing table-driven tests over writing new standalone test functions, even if existing test structs require minor adjustments to accommodate the new case. Where beneficial, convert an existing test into a table-driven test rather than duplicating test setup logic.
- **Use `testutil` Helpers:** Leverage `testutil` assertion helpers (e.g., for verifying metric outputs, linting metric registrations, and native histograms) rather than manual text scraping comparisons.

---

## Performance Work

Maintainers take performance and allocations seriously across hot paths (`WithLabelValues`, `Observe`, `Inc`, `Gather`):

- **Benchmarks Required:** Performance improvements require benchmarks demonstrating measurable improvements in execution speed and/or memory allocations.
- **Run Benchmarks:** Run benchmarks before and after the change using:
  ```bash
  go test -count=6 -benchmem -bench <benchmark_pattern> <package_path>
  ```
- **Benchstat Evidence:** Provide benchmark comparison numbers in the PR body using `benchstat` output.
- **Hot Path Allocations:** Minimize and reuse allocations in hot execution paths (slices, label maps, buffer pools).
- **Interface Contracts:** When reusing buffers or slices passed across interface boundaries, clearly document that callers must copy the contents if they need to retain them beyond the method call.

---

## Code Style & Comments

- **Go Standards:** Follow [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) and modern Go idioms (e.g., prefer `any` over `interface{}` in Go 1.18+).
- **Capitalization & Full Stops:** All comments across Go source files (including exported doc comments, unexported comments, inline comments, and test descriptions) **must start with a capital letter and end with a full stop (`.`) at the end of each sentence**. Ensure every single sentence within a comment block ends with a period (`.`).
- **Exposed Objects Doc Comments:** All exported packages, structs, interfaces, methods, functions, and constants must have complete, clear doc comments explaining their behavior and contracts.
- **State Assumptions & Contracts:** State assumptions clearly. If ownership or lifetime semantics (such as map retention vs copying) matter, document them at the interface/struct definition and not just in the implementation.
- **Linting & Linter Rules:** Run `make lint` before submitting. The project uses `golangci-lint` with specific linter rules (including `gocritic` checks such as `emptyStringTest`). Fix linter findings directly in the code rather than suppressing them with `//nolint` directives unless there is a confirmed false-positive. Use `//nolint:linter1` sparingly and always include an explanatory comment.

---

## Linking Issues & Scope Discipline

- **Issue Linking:** Use GitHub closing keywords in the PR body so linked issues close automatically upon merge:
  ```
  Fixes #18243
  ```
- **Scope Discipline:** Keep pull requests tightly scoped. Do not bundle unrelated changes together. If a refactoring step is needed before implementing a feature or bug fix, submit the refactor as a separate commit or a prerequisite PR.
- **Splitting Large PRs:** For substantial initiatives, split work into preparatory and follow-up PRs and link them using "Part of #NNNN" or "Depends on #NNNN".

---

## Documentation & CI Workflows

- **Documentation:** Ensure all new or modified code has valid comments, godoc examples, and accurate documentation. When changing documented behavior, update the relevant documentation accordingly.
- **CI / Workflow Changes:** GitHub Actions workflow files (`.github/workflows/*.yml`) must declare explicit token permissions (e.g., `statuses: write`, `contents: read`). Missing permissions can cause silent 403 failures during automated runs.
- **Multi-Module Awareness:** `client_golang` contains multiple Go modules (such as the root module `/`, `/exp`, and `/tutorials/whatsup`). When updating dependencies or CI workflows, ensure go mod updates and checks are applied across all relevant module boundaries.
