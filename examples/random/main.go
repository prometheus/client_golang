/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

/*
main.go provides a simple example of how to use this instrumentation
framework in the context of having something that emits values into
its collectors.

The emitted values correspond to uniform, normal, and exponential
distributions.
*/
package main

import (
	"flag"
	"github.com/matttproud/golang_instrumentation"
	"github.com/matttproud/golang_instrumentation/maths"
	"github.com/matttproud/golang_instrumentation/metrics"
	"math/rand"
	"net/http"
	"time"
)

var (
	listeningAddress string
)

func init() {
	flag.StringVar(&listeningAddress, "listeningAddress", ":8080", "The address to listen to requests on.")
}

func main() {
	flag.Parse()

	rpc_latency := metrics.NewHistogram(&metrics.HistogramSpecification{
		Starts:                metrics.EquallySizedBucketsFor(0, 200, 4),
		BucketBuilder:         metrics.AccumulatingBucketBuilder(metrics.EvictAndReplaceWith(10, maths.Average), 50),
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.90, 0.99},
	})

	rpc_calls := metrics.NewCounter()

	metrics := registry.NewRegistry()

	metrics.Register("rpc_latency_microseconds", "RPC latency.", registry.NilLabels, rpc_latency)
	metrics.Register("rpc_calls_total", "RPC calls.", registry.NilLabels, rpc_calls)

	go func() {
		for {
			rpc_latency.Add(map[string]string{"service": "foo"}, rand.Float64()*200)
			rpc_calls.Increment(map[string]string{"service": "foo"})

			rpc_latency.Add(map[string]string{"service": "bar"}, (rand.NormFloat64()*10.0)+100.0)
			rpc_calls.Increment(map[string]string{"service": "bar"})

			rpc_latency.Add(map[string]string{"service": "zed"}, rand.ExpFloat64())
			rpc_calls.Increment(map[string]string{"service": "zed"})

			time.Sleep(100 * time.Millisecond)
		}
	}()

	exporter := metrics.YieldExporter()

	http.Handle("/metrics.json", exporter)
	http.ListenAndServe(listeningAddress, nil)
}
