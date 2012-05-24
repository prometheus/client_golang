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
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.90, 0.99},
	})
	foo_rpc_calls := &metrics.GaugeMetric{}
	bar_rpc_latency := metrics.CreateHistogram(&metrics.HistogramSpecification{
		Starts:                metrics.EquallySizedBucketsFor(0, 200, 4),
		BucketMaker:           metrics.AccumulatingBucketBuilder(metrics.EvictAndReplaceWith(10, maths.Average), 50),
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.90, 0.99},
	})
	bar_rpc_calls := &metrics.GaugeMetric{}
	zed_rpc_latency := metrics.CreateHistogram(&metrics.HistogramSpecification{
		Starts:                metrics.EquallySizedBucketsFor(0, 200, 4),
		BucketMaker:           metrics.AccumulatingBucketBuilder(metrics.EvictAndReplaceWith(10, maths.Average), 50),
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.90, 0.99},
	})
	zed_rpc_calls := &metrics.GaugeMetric{}

	metrics := registry.NewRegistry()
	metrics.Register("rpc_latency_foo_microseconds", foo_rpc_latency)
	metrics.Register("rpc_calls_foo_total", foo_rpc_calls)
	metrics.Register("rpc_latency_bar_microseconds", bar_rpc_latency)
	metrics.Register("rpc_calls_bar_total", bar_rpc_calls)
	metrics.Register("rpc_latency_zed_microseconds", zed_rpc_latency)
	metrics.Register("rpc_calls_zed_total", zed_rpc_calls)

	go func() {
		for {
			foo_rpc_latency.Add(rand.Float64() * 200)
			foo_rpc_calls.Increment()

			bar_rpc_latency.Add((rand.NormFloat64() * 10.0) + 100.0)
			bar_rpc_calls.Increment()

			zed_rpc_latency.Add(rand.ExpFloat64())
			zed_rpc_calls.Increment()

			time.Sleep(100 * time.Millisecond)
		}
	}()

	exporter := metrics.YieldExporter()

	http.Handle("/metrics.json", exporter)
	http.ListenAndServe(":8080", nil)
}
