// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
//

package prometheus

import (
	"time"
)

// Boilerplate metrics about the metrics reporting subservice.  These are only
// exposed if the DefaultRegistry's exporter is hooked into the HTTP request
// handler.
var (
	marshalErrorCount = NewCounter()
	dumpErrorCount    = NewCounter()

	requestCount          = NewCounter()
	requestLatencyBuckets = LogarithmicSizedBucketsFor(0, 1000)
	requestLatency        = NewHistogram(&HistogramSpecification{
		Starts:                requestLatencyBuckets,
		BucketBuilder:         AccumulatingBucketBuilder(EvictAndReplaceWith(50, AverageReducer), 1000),
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.9, 0.99},
	})

	startTime = NewGauge()
)

func init() {
	startTime.Set(nil, float64(time.Now().Unix()))

	DefaultRegistry.Register("telemetry_requests_metrics_total", "A counter of the total requests made against the telemetry system.", NilLabels, requestCount)
	DefaultRegistry.Register("telemetry_requests_metrics_latency_microseconds", "A histogram of the response latency for requests made against the telemetry system.", NilLabels, requestLatency)

	DefaultRegistry.Register("instance_start_time_seconds", "The time at which the current instance started (UTC).", NilLabels, startTime)
}

// This callback accumulates the microsecond duration of the reporting
// framework's overhead such that it can be reported.
var requestLatencyAccumulator CompletionCallback = func(duration time.Duration) {
	microseconds := float64(duration / time.Microsecond)

	requestLatency.Add(nil, microseconds)
}
