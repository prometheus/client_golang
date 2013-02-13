// Copyright (c) 2013, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
//

package registry

import (
	"github.com/prometheus/client_golang/maths"
	"github.com/prometheus/client_golang/metrics"
	"time"
)

// Boilerplate metrics about the metrics reporting subservice.  These are only
// exposed if the DefaultRegistry's exporter is hooked into the HTTP request
// handler.
var (
	marshalErrorCount = metrics.NewCounter()
	dumpErrorCount    = metrics.NewCounter()

	requestCount          = metrics.NewCounter()
	requestLatencyBuckets = metrics.LogarithmicSizedBucketsFor(0, 1000)
	requestLatency        = metrics.NewHistogram(&metrics.HistogramSpecification{
		Starts:                requestLatencyBuckets,
		BucketBuilder:         metrics.AccumulatingBucketBuilder(metrics.EvictAndReplaceWith(50, maths.Average), 1000),
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.9, 0.99},
	})

	startTime = metrics.NewGauge()
)

func init() {
	startTime.Set(nil, float64(time.Now().Unix()))

	DefaultRegistry.Register("telemetry_requests_metrics_total", "A counter of the total requests made against the telemetry system.", NilLabels, requestCount)
	DefaultRegistry.Register("telemetry_requests_metrics_latency_microseconds", "A histogram of the response latency for requests made against the telemetry system.", NilLabels, requestLatency)

	DefaultRegistry.Register("instance_start_time_seconds", "The time at which the current instance started (UTC).", NilLabels, startTime)
}

// This callback accumulates the microsecond duration of the reporting
// framework's overhead such that it can be reported.
var requestLatencyAccumulator metrics.CompletionCallback = func(duration time.Duration) {
	microseconds := float64(duration / time.Microsecond)

	requestLatency.Add(nil, microseconds)
}
