// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// main.go provides a simple example of how to use this instrumentation
// framework in the context of having something that emits values into
// its collectors.

package main

import (
	"github.com/matttproud/golang_instrumentation"
	"github.com/matttproud/golang_instrumentation/maths"
	"github.com/matttproud/golang_instrumentation/metrics"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	foo_rpc_latency := metrics.CreateHistogram(&metrics.HistogramSpecification{
		Starts:                metrics.EquallySizedBucketsFor(0, 200, 4),
		BucketMaker:           metrics.AccumulatingBucketBuilder(metrics.EvictAndReplaceWith(10, maths.Average), 50),
		ReportablePercentiles: []float64{0.01, 0.5, 0.90, 0.99},
	})
	foo_rpc_calls := &metrics.GaugeMetric{}

	metrics := registry.NewRegistry()
	metrics.Register("foo_rpc_latency_ms_histogram", foo_rpc_latency)
	metrics.Register("foo_rpc_call_count", foo_rpc_calls)

	go func() {
		for {
			foo_rpc_latency.Add(rand.Float64() * 200)
			foo_rpc_calls.Increment()
			time.Sleep(500 * time.Millisecond)
		}
	}()

	exporter := metrics.YieldExporter()

	http.Handle("/metrics.json", exporter)
	http.ListenAndServe(":8080", nil)
}
