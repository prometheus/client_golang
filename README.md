# Overview
This is the [Prometheus](http://www.prometheus.io)
[Go](http://golang.org) client library.  It provides several distinct
functions, and there is separate documentation for each respective
component.  You will want to select the appropriate topic below to
continue your journey:

  1. See the [exposition library](prometheus/README.md) if you want to
     export metrics to a Prometheus server or pushgateway

  2. See the [consumption library](extraction/README.md) if you want to
     process metrics exported by a Prometheus client. (The Prometheus server
     is using that library.)

[![GoDoc](https://godoc.org/github.com/prometheus/client_golang?status.png)](https://godoc.org/github.com/prometheus/client_golang)
     
# Getting Started

  * The source code is periodically indexed: [Go Exposition Client](http://godoc.org/github.com/prometheus/client_golang).
  * All of the core developers are accessible via the [Prometheus Developers Mailinglist](https://groups.google.com/forum/?fromgroups#!forum/prometheus-developers).

# Testing
    $ go test ./...

# Continuous Integration
[![Build Status](https://secure.travis-ci.org/prometheus/client_golang.png?branch=master)]()

##  Contributing

See the contributing guidelines for the [Prometheus server](https://github.com/prometheus/prometheus/blob/master/CONTRIBUTING.md).
