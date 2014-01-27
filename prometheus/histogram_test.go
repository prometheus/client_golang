// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"reflect"
	"testing"
	"time"
)

// TODO(matt): Re-Add tests for this type.

func TestHistogram_samples(t *testing.T) {
	// there must be a better way of testing the samples produced by a histogram.
	histogram := NewHistogram(
		&HistogramSpecification{
			Starts:                EquallySizedBucketsFor(0, 10, 5),
			BucketBuilder:         AccumulatingBucketBuilder(EvictAndReplaceWith(10, AverageReducer), 50),
			ReportablePercentiles: []float64{0.01, 0.99},
			PurgeInterval:         15 * time.Minute,
		},
	)

	histogram.Add(map[string]string{"foo": "bar"}, 10)
	histogram.Add(map[string]string{"foo": "bar"}, 10)
	histogram.Add(map[string]string{"foo": "bar"}, 10)
	histogram.Add(map[string]string{"foo": "bar"}, 1)
	histogram.Add(map[string]string{"foo": "bar"}, 1)
	histogram.Add(map[string]string{"foo": "bar"}, 1)

	histogram.Add(map[string]string{"bar": "baz"}, 5)
	histogram.Add(map[string]string{"bar": "baz"}, 5)
	histogram.Add(map[string]string{"bar": "baz"}, 5)
	histogram.Add(map[string]string{"bar": "baz"}, 2)
	histogram.Add(map[string]string{"bar": "baz"}, 2)
	histogram.Add(map[string]string{"bar": "baz"}, 2)

	got := []testSample{}
	expected := []testSample{
		{"metric_sum", 33, map[string]string{"foo": "bar"}},
		{"metric_count", 6, map[string]string{"foo": "bar"}},
		{"metric", 1, map[string]string{"foo": "bar", "quantile": "0.01"}},
		{"metric", 10, map[string]string{"foo": "bar", "quantile": "0.99"}},

		{"metric_sum", 21, map[string]string{"bar": "baz"}},
		{"metric_count", 6, map[string]string{"bar": "baz"}},
		{"metric", 2, map[string]string{"bar": "baz", "quantile": "0.01"}},
		{"metric", 5, map[string]string{"bar": "baz", "quantile": "0.99"}},
	}

	histogram.samples("metric", captureSamples(&got))

	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}
