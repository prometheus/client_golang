# Prometheus Go client library

[![Build Status](https://travis-ci.org/prometheus/client_golang.svg?branch=master)](https://travis-ci.org/prometheus/client_golang) [![code-coverage](http://gocover.io/_badge/github.com/prometheus/client_golang/prometheus)](http://gocover.io/github.com/prometheus/client_golang/prometheus) [![go-doc](https://godoc.org/github.com/prometheus/client_golang/prometheus?status.svg)](https://godoc.org/github.com/prometheus/client_golang/prometheus)

This is the [Go](http://golang.org) client library for
[Prometheus](http://prometheus.io).

## Instrumenting applications

For instrumenting your Go application code with Prometheus metrics, see the
[documentation of the exposition client](https://godoc.org/github.com/prometheus/client_golang/prometheus).

## Consuming exported metrics

If you want to process metrics exported by a Prometheus client, see the
[consumption library documentation](https://godoc.org/github.com/prometheus/client_golang/extraction).
(The Prometheus server is using that library.)

# Testing

    $ go test ./...

## Contributing and community

See the [contributing guidelines](CONTRIBUTING.md) and the
[Community section](http://prometheus.io/community/) of the homepage.
