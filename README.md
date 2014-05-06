# Overview
These [Go](http://golang.org) packages are an extraction of pieces of
instrumentation code I whipped-up for a personal project that a friend of mine
and I are working on.  We were in need for some rudimentary statistics to
observe behaviors of the server's various components, so this was written.

The code here is not a verbatim copy thereof but rather a thoughtful
re-implementation should other folks need to consume and analyze such telemetry.

N.B. --- I have spent a bit of time working through the model in my head and
probably haven't elucidated my ideas as clearly as I need to.  If you examine
examples/{simple,uniform_random}/main.go and registry.go, you'll find several
examples of what types of potential instrumentation use cases this package
addresses.  There are probably numerous Go language idiomatic changes that need
to be made, but this task has been deferred for now.

# Continuous Integration
[![Build Status](https://secure.travis-ci.org/prometheus/client_golang.png?branch=master)](http://travis-ci.org/prometheus/client_golang)

# Documentation
Please read the [generated documentation](http://go.pkgdoc.org/github.com/prometheus/client_golang)
for the project's documentation from source code.

# Basic Overview
## Metrics
A metric is a measurement mechanism.

### Gauge
A _Gauge_ is a metric that exposes merely an instantaneous value or some
snapshot thereof.

### Counter
A _Counter_ is a metric that exposes merely a sum or tally of things.

### Histogram
A _Histogram_ is a metric that captures events or samples into _Buckets_.  It
exposes its values via percentile estimations.

#### Buckets
A _Bucket_ is a generic container that collects samples and their values.  It
prescribes no behavior on its own aside from merely accepting a value,
leaving it up to the concrete implementation to what to do with the injected
values.

##### Accumulating Bucket
An _Accumulating Bucket_ is a _Bucket_ that appends the new sample to a queue
such that the eldest values are evicted according to a given policy.

###### Eviction Policies
Once an _Accumulating Bucket_ reaches capacity, its eviction policy is invoked.
This reaps the oldest N objects subject to certain behavior.

####### Remove Oldest
This merely removes the oldest N items without performing some aggregation
replacement operation on them.

####### Aggregate Oldest
This removes the oldest N items while performing some summary aggregation
operation thereupon, which is then appended to the list in the former values'
place.

##### Tallying Bucket
A _Tallying Bucket_ differs from an _Accumulating Bucket_ in that it never
stores any of the values emitted into it but rather exposes a simplied summary
representation thereof.  For instance, if a values therein is requested,
it may situationally emit a minimum, maximum, an average, or any other
reduction mechanism requested.

# Getting Started

  * The source code is periodically indexed: [Go Exposition Client](http://godoc.org/github.com/prometheus/client_golang).
  * All of the core developers are accessible via the [Prometheus Developers Mailinglist](https://groups.google.com/forum/?fromgroups#!forum/prometheus-developers).


# Testing
This package employs [gocheck](http://labix.org/gocheck) for testing.  Please
ensure that all tests pass by running the following from the project root:

    $ go test ./...

The use of gocheck is summarily being phased out; however, old tests that use it
still exist.

# Contributing

Same as for the `prometheus/prometheus` repository, we are using
Gerrit to manage reviews of pull-requests for this repository. See
[`CONTRIBUTING.md`](https://github.com/prometheus/prometheus/blob/master/CONTRIBUTING.md)
in the `prometheus/prometheus` repository for details (but replace the
`prometheus` repository name by `client_golang`).

Please try to avoid warnings flagged by [`go
vet`](https://godoc.org/code.google.com/p/go.tools/cmd/vet) and by
[`golint`](https://github.com/golang/lint), and pay attention to the
[Go Code Review
Comments](https://code.google.com/p/go-wiki/wiki/CodeReviewComments) and the _Formatting and style_ section of Peter Bourgon's [Go: Best Practices for Production Environments](http://peter.bourgon.org/go-in-production/#formatting-and-style).
