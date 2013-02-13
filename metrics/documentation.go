// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The metrics package provides general descriptors for the concept of
// exportable metrics.

// accumulating_bucket.go provides a histogram bucket type that accumulates
// elements until a given capacity and enacts a given eviction policy upon
// such a condition.

// accumulating_bucket_test.go provides a test complement for the
// accumulating_bucket_go module.

// eviction.go provides several histogram bucket eviction strategies.

// eviction_test.go provides a test complement for the eviction.go module.

// gauge.go provides a scalar metric that one can monitor.  It is useful for
// certain cases, such as instantaneous temperature.

// gauge_test.go provides a test complement for the gauge.go module.

// histogram.go provides a basic histogram metric, which can accumulate scalar
// event values or samples.  The underlying histogram implementation is designed
// to be performant in that it accepts tolerable inaccuracies.

// histogram_test.go provides a test complement for the histogram.go module.

// metric.go provides fundamental interface expectations for the various
// metrics.

// metrics_test.go provides a test suite for all tests in the metrics package
// hierarchy.  It employs the gocheck framework for test scaffolding.

// tallying_bucket.go provides a histogram bucket type that aggregates tallies
// of events that fall into its ranges versus a summary of the values
// themselves.

// tallying_bucket_test.go provides a test complement for the
// tallying_bucket.go module.

// timer.go provides a scalar metric that times how long a given event takes.

// timer_test.go provides a test complement for the timer.go module.
package metrics
