# Overview
This is the [Prometheus](http://www.prometheus.io) telemetric
instrumentation client [Go](http://golang.org) client library.  It
enable authors to define process-space metrics for their servers and
expose them through a web services interface for extraction,
aggregation, and a whole slew of other post processing techniques.

# Installing
    $ go get github.com/prometheus/client_golang/prometheus

# Example
```go
package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	indexed = prometheus.NewCounter(prometheus.CounterDesc{prometheus.Desc{
		Namespace: "my_company",
		Subsystem: "indexer",
		Name:      "documents_indexed",
		Help:      "The number of documents indexed.",
	}})
	searched = prometheus.NewCounter(prometheus.CounterDesc{prometheus.Desc{
		Namespace: "my_company",
		Subsystem: "root",
		Name:      "documents_searched",
		Help:      "The number of user queries to the index."}})
)

func main() {
	http.Handle("/metrics", prometheus.Handler)

	indexed.Inc()
	searched.Set(5)

	http.ListenAndServe(":8080", nil)
}

func init() {
	prometheus.MustRegister(indexed)
	prometheus.MustRegister(searched)
}
```

# Documentation
  * Auto-generated godoc for [Go Exposition Client]
    (http://godoc.org/github.com/prometheus/client_golang/prometheus).
