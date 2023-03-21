# Prometheus Go client library

[![CircleCI](https://circleci.com/gh/prometheus/client_golang/tree/main.svg?style=svg)](https://circleci.com/gh/prometheus/client_golang/tree/main)
[![Go Report Card](https://goreportcard.com/badge/github.com/prometheus/client_golang)](https://goreportcard.com/report/github.com/prometheus/client_golang)
[![Go Reference](https://pkg.go.dev/badge/github.com/prometheus/client_golang.svg)](https://pkg.go.dev/github.com/prometheus/client_golang)
[![Slack](https://img.shields.io/badge/join%20slack-%23prometheus--client_golang-brightgreen.svg)](https://slack.cncf.io/)

This is the [Go](http://golang.org) client library for
[Prometheus](http://prometheus.io). It has two separate parts, one for
instrumenting application code, and one for creating clients that talk to the
Prometheus HTTP API.

__This library requires Go1.18 or later.__

## Important note about releases and stability

This repository generally follows [Semantic
Versioning](https://semver.org/). However, the API client in
prometheus/client_golang/api/… is still considered experimental. Breaking
changes of the API client will _not_ trigger a new major release. The same is
true for selected other new features explicitly marked as **EXPERIMENTAL** in
CHANGELOG.md.

Features that require breaking changes in the stable parts of the repository
are being batched up and tracked in the [v2
milestone](https://github.com/prometheus/client_golang/milestone/2). The v2
development happens in a [separate
branch](https://github.com/prometheus/client_golang/tree/dev-v2) for the time
being. v2 releases off that branch will happen once sufficient stability is
reached. In view of the widespread use of this repository, v1 and v2 will
coexist for a while to enable a convenient transition.

## Instrumenting applications

[![Go Reference](https://pkg.go.dev/badge/github.com/prometheus/client_golang/prometheus.svg)](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus)

The
[`prometheus` directory](https://github.com/prometheus/client_golang/tree/main/prometheus)
contains the instrumentation library. See the
[guide](https://prometheus.io/docs/guides/go-application/) on the Prometheus
website to learn more about instrumenting applications.

The
[`examples` directory](https://github.com/prometheus/client_golang/tree/main/examples)
contains simple examples of instrumented code.

## Client for the Prometheus HTTP API

[![Go Reference](https://pkg.go.dev/badge/github.com/prometheus/client_golang/api.svg)](https://pkg.go.dev/github.com/prometheus/client_golang/api)

The
[`api/prometheus` directory](https://github.com/prometheus/client_golang/tree/main/api/prometheus)
contains the client for the
[Prometheus HTTP API](http://prometheus.io/docs/querying/api/). It allows you
to write Go applications that query time series data from a Prometheus
server. It is still in alpha stage.

## Where is `model`, `extraction`, and `text`?

The `model` packages has been moved to
[`prometheus/common/model`](https://github.com/prometheus/common/tree/main/model).

The `extraction` and `text` packages are now contained in
[`prometheus/common/expfmt`](https://github.com/prometheus/common/tree/main/expfmt).

## Contributing and community

See the [contributing guidelines](CONTRIBUTING.md) and the
[Community section](http://prometheus.io/community/) of the homepage.

`client_golang` community is also present on the CNCF Slack `#prometheus-client_golang`.

### For Maintainers: Release Process

To cut a minor version:

1. Create a new branch `release-<major>.<minor>` on top of the `main` commit you want to cut the version from and push it.
2. Create a new branch on top of the release branch, e.g. `<yourname>/cut-<major>.<minor>.<patch>`,
3. Change the `VERSION` file.
4. Update `CHANGELOG` (only user-impacting changes to mention).
5. Create PR, and get it reviewed.
6. Once merged, create a release with the `release-<major>.<minor>` tag on GitHub with the `<version> / <date>` title.
7. Announce on the prometheus-announce mailing list, slack and Twitter.
8. Merge the release branch back to the `main` using the "merge without squashing" approach (!).

> NOTE: In case of merge conflicts, you can checkout the release branch in a new branch, e.g. <yourname>/resolve-conflicts`, fix the merge problems there, and then do a PR into main from the new branch. In that way, you still get all the commits in the release branch back into `main`, but leave the release branch alone.

To cut the patch version:

1. Create a branch on top of the release branch you want to use.
2. Cherry-pick the fixes from the `main` branch (or add new commits) to fix critical bugs for that patch release.
3. Follow steps 3-8 as above.
