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

## How to fill the PR template

### Describe your PR

In this section, provide a clear and concise description of what your PR does. This helps reviewers understand the purpose and context of your changes.

### What type of PR is this?

Indicate the type of PR by adding one of the following options:

- /kind chore
- /kind cleanup
- /kind fix
- /kind bugfix
- /kind enhancement
- /kind feature
- /kind feat
- /kind docs
- /kind NONE

If this change should not appear in the changelog, use `/kind NONE`.

### Changelog Entry

Include a brief summary of your change for the changelog. This helps users understand what has been modified, added, or fixed in the project. If your change should not appear in the changelog, write `NONE`. Make sure to add only user-facing changes.
