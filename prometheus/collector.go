// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

// MetricsCollector is the interface implemented by anything that can be used by
// Prometheus to collect metrics. The stock metrics provided by this package
// (like Gauge, Counter, Summary) are also MetricCollectors (which only ever
// collect one metric, namely itself). An implementer of MetricsCollector may,
// however, collect multiple metrics in a coordinated fashion and/or create
// metrics on the fly. Examples for collectors already implemented in this
// library are the multi-dimensional metrics (i.e. metrics with variable lables)
// like GaugeVec or SummaryVec and the ExpvarCollector.
type MetricsCollector interface {
	// DescribeMetrics returns the super-set of all possible descriptors of
	// metrics collected by this MetricsCollector. The returned descriptors
	// fulfill the consistency and uniqueness requirements described in the
	// Desc documentation. This method idempotently returns the same
	// descriptors throughout the lifetime of the Metric.
	DescribeMetrics() []*Desc
	// CollectMetrics is called by Prometheus when collecting metrics. The
	// descriptor of each returned metric is one of those returned by
	// DescribeMetrics. Each returned metric differs from each other eithen
	// in its descriptor or in its variable label values. The returned
	// metrics are sorted consistently. This method may be called
	// concurrently and must therefore be implemented in a concurrency safe
	// way. Blocking occurs at the expense of total performance of rendering
	// all registered metrics.  Ideally MetricsCollector implementations
	// should support concurrent readers.
	CollectMetrics() []Metric
}

// SelfCollector implements MetricsCollector for a single metric so that that
// metric collects itself. Add it as an anonymous field to a struct that
// implements Metric, and set MetricSlice and DescSlice appropriately.
type SelfCollector struct {
	MetricSlice []Metric
	DescSlice   []*Desc
}

func (c *SelfCollector) DescribeMetrics() []*Desc {
	return c.DescSlice
}

func (c *SelfCollector) CollectMetrics() []Metric {
	return c.MetricSlice
}
