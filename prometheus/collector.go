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
// metrics on the fly.
type MetricsCollector interface {
	// DescribeMetrics returns the super-set of all possible descriptors of
	// metrics collected by this MetricsCollector. The returned descriptors
	// fulfill the consistency and uniqueness requirements described in the
	// Desc documentation. This method idempotently returns the same
	// descriptors throughout the lifetime of the Metric.
	DescribeMetrics() []*Desc
	// CollectMetrics is called by Prometheus when collecting metrics. Each
	// returned metric has a different descriptor, and each descriptor is
	// one of those returned by DescribeMetrics. This method may be called
	// concurrently and must therefore be implemented in a concurrency safe
	// way. Blocking occurs at the expense of total performance of rendering
	// all registered metrics.  Ideally MetricsCollector implementations
	// should support concurrent readers.
	CollectMetrics() []Metric
}

// SelfCollector implements MetricsCollector for a single metric so that that
// metric collects itself. Add it as an anonymous field to a struct that
// implements Metric.
type SelfCollector struct {
	Self Metric
}

func (c *SelfCollector) DescribeMetrics() []*Desc {
	return []*Desc{c.Self.Desc()}
}

func (c *SelfCollector) CollectMetrics() []Metric {
	return []Metric{c.Self}
}
