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

## Managing dependencies

This repository uses Go modules and does not commit a `vendor` directory. Add
dependencies to the module that uses them: the root module, `exp`, or a
tutorial module with its own `go.mod` file.

* Prefer stable, well-maintained dependencies. Discuss the use of a
  pre-release, unstable, or otherwise unusual dependency with maintainers
  before opening a PR.
* Add or update a dependency with `go get module/path@version`, then run
  `go mod tidy`. Commit the resulting `go.mod` and `go.sum` changes.
* Do not manually add or remove `// indirect` comments. `go mod tidy`
  determines whether a dependency is direct based on imports in the module.
* Dependencies imported only by tests or examples still belong in that
  module's `go.mod`. Use the module containing the test or example rather than
  adding a dependency to the root module unnecessarily.
