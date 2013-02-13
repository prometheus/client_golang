// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// main.go provides a simple example of how to use this instrumentation
// framework in the context of having something that emits values into
// its collectors.
//
// The emitted values correspond to uniform, normal, and exponential
// distributions.
package main

import (
	"flag"
	"github.com/prometheus/client_golang"
	"github.com/prometheus/client_golang/maths"
	"github.com/prometheus/client_golang/metrics"
	"math/rand"
	"net/http"
	"time"
)

var (
	listeningAddress string

	barDomain float64
	barMean   float64
	fooDomain float64

	// Create a histogram to track fictitious interservice RPC latency for three
	// distinct services.
	rpc_latency = metrics.NewHistogram(&metrics.HistogramSpecification{
		// Four distinct histogram buckets for values:
		// - equally-sized,
		// - 0 to 50, 50 to 100, 100 to 150, and 150 to 200.
		Starts: metrics.EquallySizedBucketsFor(0, 200, 4),
		// Create histogram buckets using an accumulating bucket, a bucket that
		// holds sample values subject to an eviction policy:
		// - 50 elements are allowed per bucket.
		// - Once 50 have been reached, the bucket empties 10 elements, averages the
		//   evicted elements, and re-appends that back to the bucket.
		BucketBuilder: metrics.AccumulatingBucketBuilder(metrics.EvictAndReplaceWith(10, maths.Average), 50),
		// The histogram reports percentiles 1, 5, 50, 90, and 99.
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.90, 0.99},
	})

	rpc_calls = metrics.NewCounter()

	// If for whatever reason you are resistant to the idea of having a static
	// registry for metrics, which is a really bad idea when using Prometheus-
	// enabled library code, you can create your own.
	customRegistry = registry.NewRegistry()
)

func init() {
	flag.StringVar(&listeningAddress, "listeningAddress", ":8080", "The address to listen to requests on.")
	flag.Float64Var(&fooDomain, "random.fooDomain", 200, "The domain for the random parameter foo.")
	flag.Float64Var(&barDomain, "random.barDomain", 10, "The domain for the random parameter bar.")
	flag.Float64Var(&barMean, "random.barMean", 100, "The mean for the random parameter bar.")
}

func main() {
	flag.Parse()

	go func() {
		for {
			rpc_latency.Add(map[string]string{"service": "foo"}, rand.Float64()*fooDomain)
			rpc_calls.Increment(map[string]string{"service": "foo"})

			rpc_latency.Add(map[string]string{"service": "bar"}, (rand.NormFloat64()*barDomain)+barMean)
			rpc_calls.Increment(map[string]string{"service": "bar"})

			rpc_latency.Add(map[string]string{"service": "zed"}, rand.ExpFloat64())
			rpc_calls.Increment(map[string]string{"service": "zed"})

			time.Sleep(100 * time.Millisecond)
		}
	}()

	http.Handle(registry.ExpositionResource, customRegistry.Handler())
	http.ListenAndServe(listeningAddress, nil)
}

func init() {
	customRegistry.Register("rpc_latency_microseconds", "RPC latency.", registry.NilLabels, rpc_latency)
	customRegistry.Register("rpc_calls_total", "RPC calls.", registry.NilLabels, rpc_calls)
}
