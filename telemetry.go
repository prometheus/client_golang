/*
Copyright (c) 2013, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style license that can be found in
the LICENSE file.
*/

package registry

import (
	"github.com/matttproud/golang_instrumentation/maths"
	"github.com/matttproud/golang_instrumentation/metrics"
	"time"
)

/*
Boilerplate metrics about the metrics reporting subservice.  These are only
exposed if the DefaultRegistry's exporter is hooked into the HTTP request
handler.
*/
var (
	// TODO(matt): Refresh these names to support namespacing.

	requestCount                          *metrics.CounterMetric = &metrics.CounterMetric{}
	requestLatencyLogarithmicBuckets      []float64              = metrics.LogarithmicSizedBucketsFor(0, 1000)
	requestLatencyEqualBuckets            []float64              = metrics.EquallySizedBucketsFor(0, 1000, 10)
	requestLatencyLogarithmicAccumulating *metrics.Histogram     = metrics.CreateHistogram(&metrics.HistogramSpecification{
		Starts:                requestLatencyLogarithmicBuckets,
		BucketMaker:           metrics.AccumulatingBucketBuilder(metrics.EvictAndReplaceWith(50, maths.Average), 1000),
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.9, 0.99},
	})
	requestLatencyEqualAccumulating *metrics.Histogram = metrics.CreateHistogram(&metrics.HistogramSpecification{
		Starts:                requestLatencyEqualBuckets,
		BucketMaker:           metrics.AccumulatingBucketBuilder(metrics.EvictAndReplaceWith(50, maths.Average), 1000),
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.9, 0.99},
	})
	requestLatencyLogarithmicTallying *metrics.Histogram = metrics.CreateHistogram(&metrics.HistogramSpecification{
		Starts:                requestLatencyLogarithmicBuckets,
		BucketMaker:           metrics.TallyingBucketBuilder,
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.9, 0.99},
	})
	requestLatencyEqualTallying *metrics.Histogram = metrics.CreateHistogram(&metrics.HistogramSpecification{
		Starts:                requestLatencyEqualBuckets,
		BucketMaker:           metrics.TallyingBucketBuilder,
		ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.9, 0.99},
	})

	startTime *metrics.GaugeMetric = &metrics.GaugeMetric{}
)

func init() {
	startTime.Set(float64(time.Now().Unix()))

	DefaultRegistry.Register("requests_metrics_total", "A counter of the total requests made against the telemetry system.", NilLabels, requestCount)
	DefaultRegistry.Register("requests_metrics_latency_logarithmic_accumulating_microseconds", "A histogram of the response latency for requests made against the telemetry system.", NilLabels, requestLatencyLogarithmicAccumulating)
	DefaultRegistry.Register("requests_metrics_latency_equal_accumulating_microseconds", "A histogram of the response latency for requests made against the telemetry system.", NilLabels, requestLatencyEqualAccumulating)
	DefaultRegistry.Register("requests_metrics_latency_logarithmic_tallying_microseconds", "A histogram of the response latency for requests made against the telemetry system.", NilLabels, requestLatencyLogarithmicTallying)
	DefaultRegistry.Register("request_metrics_latency_equal_tallying_microseconds", "A histogram of the response latency for requests made against the telemetry system.", NilLabels, requestLatencyEqualTallying)

	DefaultRegistry.Register("instance_start_time_seconds", "The time at which the current instance started (UTC).", NilLabels, startTime)
}
