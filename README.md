# Overview
This is the [Prometheus](http://www.prometheus.io)
[Go](http://golang.org) client library.  It provides several distinct
functions, and there is separate documentation for each respective
component.  You will want to select the appropriate topic below to
continue your journey:

  1. Telemetric instrumentation of servers written in Go and
     exposition through a web services interface.

     See the [exposition library](prometheus/README.md) for more.

  2. Processing of remote telemetric data.

     See the [consumption library](extraction/README.md) for more.

# Getting Started

  * The source code is periodically indexed: [Go Exposition Client](http://godoc.org/github.com/prometheus/client_golang).
  * All of the core developers are accessible via the [Prometheus Developers Mailinglist](https://groups.google.com/forum/?fromgroups#!forum/prometheus-developers).

# Testing
    $ go test ./...

# Continuous Integration
[![Build Status](https://secure.travis-ci.org/prometheus/client_golang.png?branch=master)](http://travis-ci.org/prometheus/client_golang)
