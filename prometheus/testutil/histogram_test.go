// Copyright 2025 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutil

import (
	"math"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"

	"github.com/prometheus/client_golang/prometheus"
)

// newNativeHistogramRegistry registers a native histogram named
// "test_native_histogram" with bucket factor 1.1 (schema 3), observes the given
// values, and returns the registry.
func newNativeHistogramRegistry(t *testing.T, observations ...float64) *prometheus.Registry {
	t.Helper()
	reg := prometheus.NewRegistry()
	h := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                        "test_native_histogram",
		Help:                        "A native histogram for testing.",
		NativeHistogramBucketFactor: 1.1,
	})
	reg.MustRegister(h)
	for _, v := range observations {
		h.Observe(v)
	}
	return reg
}

func TestGatherAndAssertNativeHistogramExists(t *testing.T) {
	reg := newNativeHistogramRegistry(t, 0, 1, 2, 3)
	if err := GatherAndAssertNativeHistogramExists(reg, "test_native_histogram", nil); err != nil {
		t.Errorf("expected native histogram to exist: %v", err)
	}
	if err := GatherAndAssertNativeHistogramExists(reg, "does_not_exist", nil); err == nil {
		t.Error("expected error for missing metric family")
	}

	// A classic (non-native) histogram must be rejected.
	classicReg := prometheus.NewRegistry()
	classic := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "classic_histogram",
		Help:    "A classic histogram.",
		Buckets: []float64{1, 2, 3},
	})
	classicReg.MustRegister(classic)
	classic.Observe(1)
	if err := GatherAndAssertNativeHistogramExists(classicReg, "classic_histogram", nil); err == nil {
		t.Error("expected error for classic histogram")
	}
}

func TestGatherAndAssertNativeHistogramCount(t *testing.T) {
	reg := newNativeHistogramRegistry(t, 0, 1, 2, 3)
	if err := GatherAndAssertNativeHistogramCount(reg, "test_native_histogram", nil, 4); err != nil {
		t.Errorf("unexpected error for matching count: %v", err)
	}
	if err := GatherAndAssertNativeHistogramCount(reg, "test_native_histogram", nil, 5); err == nil {
		t.Error("expected error for mismatching count")
	}
}

func TestGatherAndAssertNativeHistogramSum(t *testing.T) {
	reg := newNativeHistogramRegistry(t, 0, 1, 2, 3) // sum == 6
	if err := GatherAndAssertNativeHistogramSum(reg, "test_native_histogram", nil, 6, 0); err != nil {
		t.Errorf("unexpected error for exact sum: %v", err)
	}
	if err := GatherAndAssertNativeHistogramSum(reg, "test_native_histogram", nil, 6.4, 0.5); err != nil {
		t.Errorf("unexpected error for sum within tolerance: %v", err)
	}
	if err := GatherAndAssertNativeHistogramSum(reg, "test_native_histogram", nil, 7, 0.5); err == nil {
		t.Error("expected error for sum outside tolerance")
	}
}

func TestGatherAndAssertNativeHistogramIntervalCount(t *testing.T) {
	const eps = 1e-9

	// Observations at powers of two are exact bucket upper bounds for schema 3.
	reg := newNativeHistogramRegistry(t, 1, 2, 4, 8)
	if err := GatherAndAssertNativeHistogramIntervalCount(reg, "test_native_histogram", nil, 0, 100, 4, eps); err != nil {
		t.Errorf("whole range should count all observations: %v", err)
	}
	// (1, 4] captures the observations at 2 and 4; bounds are exact bucket edges.
	// The observation at exactly 1 falls in bucket (2^(-1/8), 1], whose upper edge
	// is the lower bound, so it belongs to the preceding interval and is excluded.
	if err := GatherAndAssertNativeHistogramIntervalCount(reg, "test_native_histogram", nil, 1, 4, 2, eps); err != nil {
		t.Errorf("interval (1,4] should count 2 observations: %v", err)
	}

	// Interpolation: 100 observations into a single bucket (2^(7/8), 2].
	interpReg := prometheus.NewRegistry()
	hi := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                        "interp_histogram",
		Help:                        "A native histogram for interpolation testing.",
		NativeHistogramBucketFactor: 1.1,
	})
	interpReg.MustRegister(hi)
	for range 100 {
		hi.Observe(2.0)
	}
	bucketLower := nativeHistogramUpperBound(3, 7) // 2^(7/8)
	bucketMid := math.Exp2(15.0 / 16.0)            // geometric mean of (2^(7/8), 2^(8/8)]
	// Up to the geometric mean: half the bucket (log-uniform interpolation).
	if err := GatherAndAssertNativeHistogramIntervalCount(interpReg, "interp_histogram", nil, bucketLower, bucketMid, 50, 0.5); err != nil {
		t.Errorf("half-bucket interpolation should be ~50: %v", err)
	}
	// The whole bucket: all 100, exact bounds, no interpolation.
	if err := GatherAndAssertNativeHistogramIntervalCount(interpReg, "interp_histogram", nil, bucketLower, 2, 100, eps); err != nil {
		t.Errorf("full bucket should count 100: %v", err)
	}

	// Negative observations: 100 into the negative bucket [-2, -2^(7/8)).
	negReg := prometheus.NewRegistry()
	hn := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                        "neg_histogram",
		Help:                        "A native histogram for negative-bucket testing.",
		NativeHistogramBucketFactor: 1.1,
	})
	negReg.MustRegister(hn)
	for range 100 {
		hn.Observe(-2.0)
	}
	if err := GatherAndAssertNativeHistogramIntervalCount(negReg, "neg_histogram", nil, -100, 0, 100, eps); err != nil {
		t.Errorf("negative whole range should count 100: %v", err)
	}

	// Quarter of the way up bucket (2^(7/8), 2] on the log scale must count ~25.
	bucketQuarter := math.Exp2(29.0 / 32.0) // 2^(7/8 + (1/4)(1/8))
	if err := GatherAndAssertNativeHistogramIntervalCount(interpReg, "interp_histogram", nil, bucketLower, bucketQuarter, 25, 0.5); err != nil {
		t.Errorf("quarter-bucket interpolation should be ~25: %v", err)
	}

	// The lower (more negative) half of [-2, -2^(7/8)) must count ~50, pinning the
	// direction of the negative-bucket interpolation.
	negMid := -math.Exp2(15.0 / 16.0) // geometric mean of the magnitudes
	if err := GatherAndAssertNativeHistogramIntervalCount(negReg, "neg_histogram", nil, -2, negMid, 50, 0.5); err != nil {
		t.Errorf("half negative bucket interpolation should be ~50: %v", err)
	}

	// The lower bound is exclusive: 2.0 lands on the upper edge of (2^(7/8), 2], so
	// (2, 100] excludes it while (2^(7/8), 2] includes it.
	edgeReg := prometheus.NewRegistry()
	he := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                        "edge_histogram",
		Help:                        "A native histogram for boundary testing.",
		NativeHistogramBucketFactor: 1.1,
	})
	edgeReg.MustRegister(he)
	he.Observe(2.0)
	if err := GatherAndAssertNativeHistogramIntervalCount(edgeReg, "edge_histogram", nil, 2, 100, 0, eps); err != nil {
		t.Errorf("lower bound should be exclusive, want 0 in (2, 100]: %v", err)
	}
	if err := GatherAndAssertNativeHistogramIntervalCount(edgeReg, "edge_histogram", nil, bucketLower, 2, 1, eps); err != nil {
		t.Errorf("interval (2^(7/8), 2] should count 1: %v", err)
	}
}

func TestGatherAndAssertNativeHistogramNotAHistogram(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "a_counter",
		Help: "A counter, not a histogram.",
	})
	reg.MustRegister(c)
	c.Inc()
	if err := GatherAndAssertNativeHistogramExists(reg, "a_counter", nil); err == nil {
		t.Error("expected error when the named metric is not a histogram")
	}
}

func TestGatherAndAssertNativeHistogramZeroObservations(t *testing.T) {
	const eps = 1e-9

	// A native histogram with no observations still exists and reports count 0.
	empty := newNativeHistogramRegistry(t)
	if err := GatherAndAssertNativeHistogramExists(empty, "test_native_histogram", nil); err != nil {
		t.Errorf("empty native histogram should still exist: %v", err)
	}
	if err := GatherAndAssertNativeHistogramCount(empty, "test_native_histogram", nil, 0); err != nil {
		t.Errorf("empty native histogram count should be 0: %v", err)
	}
	if err := GatherAndAssertNativeHistogramIntervalCount(empty, "test_native_histogram", nil, -100, 100, 0, eps); err != nil {
		t.Errorf("empty native histogram interval should be 0: %v", err)
	}

	// An observation of exactly 0 lands in the zero bucket and is counted by an
	// interval spanning it.
	zero := newNativeHistogramRegistry(t, 0)
	if err := GatherAndAssertNativeHistogramIntervalCount(zero, "test_native_histogram", nil, -1, 1, 1, eps); err != nil {
		t.Errorf("observation at 0 should be counted in (-1, 1]: %v", err)
	}
}

// floatGatherer returns the given metric families verbatim, letting tests feed
// hand-built float native histograms through the public API (client_golang has
// no public float-histogram API to produce them).
type floatGatherer struct{ mfs []*dto.MetricFamily }

func (g floatGatherer) Gather() ([]*dto.MetricFamily, error) { return g.mfs, nil }

func TestGatherAndAssertNativeHistogramFloat(t *testing.T) {
	const eps = 1e-9

	// Float native histogram, schema 0: a zero bucket with float count 2 and one
	// positive bucket at index 1 covering (1, 2] with absolute float count 3.
	// SampleCountFloat, ZeroCountFloat, and PositiveCount exercise the float paths.
	g := floatGatherer{mfs: []*dto.MetricFamily{{
		Name: proto.String("float_native_histogram"),
		Type: dto.MetricType_HISTOGRAM.Enum(),
		Metric: []*dto.Metric{{
			Histogram: &dto.Histogram{
				SampleCountFloat: proto.Float64(5),
				SampleSum:        proto.Float64(7.5),
				Schema:           proto.Int32(0),
				ZeroThreshold:    proto.Float64(0.5),
				ZeroCountFloat:   proto.Float64(2),
				PositiveSpan: []*dto.BucketSpan{
					{Offset: proto.Int32(1), Length: proto.Uint32(1)},
				},
				PositiveCount: []float64{3},
			},
		}},
	}}}

	if err := GatherAndAssertNativeHistogramExists(g, "float_native_histogram", nil); err != nil {
		t.Errorf("float histogram should exist: %v", err)
	}
	if err := GatherAndAssertNativeHistogramCount(g, "float_native_histogram", nil, 5); err != nil {
		t.Errorf("float histogram count: %v", err)
	}
	if err := GatherAndAssertNativeHistogramSum(g, "float_native_histogram", nil, 7.5, eps); err != nil {
		t.Errorf("float histogram sum: %v", err)
	}
	// Whole range counts all 5 observations (2 in the zero bucket, 3 positive).
	if err := GatherAndAssertNativeHistogramIntervalCount(g, "float_native_histogram", nil, -100, 100, 5, eps); err != nil {
		t.Errorf("float histogram whole-range interval: %v", err)
	}
	// (1, 2] captures only the positive bucket's 3 observations (exact edges).
	if err := GatherAndAssertNativeHistogramIntervalCount(g, "float_native_histogram", nil, 1, 2, 3, eps); err != nil {
		t.Errorf("float histogram interval (1, 2]: %v", err)
	}
}

func TestGatherAndAssertNativeHistogramLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	hv := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:                        "vec_histogram",
		Help:                        "A native histogram vector.",
		NativeHistogramBucketFactor: 1.1,
	}, []string{"code"})
	reg.MustRegister(hv)
	hv.WithLabelValues("200").Observe(1)
	hv.WithLabelValues("200").Observe(2)
	hv.WithLabelValues("404").Observe(1)

	if err := GatherAndAssertNativeHistogramCount(reg, "vec_histogram", prometheus.Labels{"code": "200"}, 2); err != nil {
		t.Errorf("selecting code=200 should match 2 observations: %v", err)
	}
	if err := GatherAndAssertNativeHistogramCount(reg, "vec_histogram", prometheus.Labels{"code": "404"}, 1); err != nil {
		t.Errorf("selecting code=404 should match 1 observation: %v", err)
	}
	// Ambiguous: more than one series and no label selector.
	if err := GatherAndAssertNativeHistogramExists(reg, "vec_histogram", nil); err == nil {
		t.Error("expected ambiguity error when multiple series match")
	}
	// No series matches the selector.
	if err := GatherAndAssertNativeHistogramExists(reg, "vec_histogram", prometheus.Labels{"code": "500"}); err == nil {
		t.Error("expected error when no series matches the labels")
	}
}

func TestNativeHistogramUpperBound(t *testing.T) {
	const eps = 1e-12
	for _, tc := range []struct {
		schema int32
		index  int
		want   float64
	}{
		{schema: 0, index: 0, want: 1},
		{schema: 0, index: 1, want: 2},
		{schema: 0, index: -1, want: 0.5},
		{schema: 0, index: 3, want: 8},
		{schema: 3, index: 0, want: 1},
		{schema: 3, index: 8, want: 2},  // 2^(8/8)
		{schema: 3, index: 16, want: 4}, // 2^(16/8)
		{schema: 3, index: 1, want: math.Exp2(1.0 / 8.0)},
		{schema: -1, index: 0, want: 1},
		{schema: -1, index: 1, want: 4}, // base 4
		{schema: -1, index: 2, want: 16},
		{schema: -1, index: -1, want: 0.25},
		{schema: -2, index: 1, want: 16}, // base 16
	} {
		got := nativeHistogramUpperBound(tc.schema, tc.index)
		if math.Abs(got-tc.want) > eps*math.Max(1, math.Abs(tc.want)) {
			t.Errorf("nativeHistogramUpperBound(%d, %d) = %v, want %v", tc.schema, tc.index, got, tc.want)
		}
	}
}

func TestDecodeSpanIndices(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spans []*dto.BucketSpan
		want  []int
	}{
		{
			name: "single bucket at origin",
			spans: []*dto.BucketSpan{
				{Offset: proto.Int32(0), Length: proto.Uint32(1)},
			},
			want: []int{0},
		},
		{
			// The canonical client_golang fixture: observing {1,2,3} at schema 3.
			name: "gapped single buckets",
			spans: []*dto.BucketSpan{
				{Offset: proto.Int32(0), Length: proto.Uint32(1)},
				{Offset: proto.Int32(7), Length: proto.Uint32(1)},
				{Offset: proto.Int32(4), Length: proto.Uint32(1)},
			},
			want: []int{0, 8, 13},
		},
		{
			name: "multi-bucket spans with gap",
			spans: []*dto.BucketSpan{
				{Offset: proto.Int32(2), Length: proto.Uint32(3)},
				{Offset: proto.Int32(5), Length: proto.Uint32(2)},
			},
			want: []int{2, 3, 4, 10, 11},
		},
		{
			name: "negative first offset",
			spans: []*dto.BucketSpan{
				{Offset: proto.Int32(-3), Length: proto.Uint32(2)},
			},
			want: []int{-3, -2},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeSpanIndices(tc.spans)
			if len(got) != len(tc.want) {
				t.Fatalf("decodeSpanIndices() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("decodeSpanIndices() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestDecodeNativeHistogram reconstructs buckets from a histogram produced by the
// real client and checks the bounds and counts.
func TestDecodeNativeHistogram(t *testing.T) {
	// Integer native histogram, schema 3, observing 0,1,2,3.
	// Expected: zero bucket count 1, positive buckets at indices 0,8,13 each count 1.
	h := &dto.Histogram{
		SampleCount:   proto.Uint64(4),
		SampleSum:     proto.Float64(6),
		Schema:        proto.Int32(3),
		ZeroThreshold: proto.Float64(2.938735877055719e-39),
		ZeroCount:     proto.Uint64(1),
		PositiveSpan: []*dto.BucketSpan{
			{Offset: proto.Int32(0), Length: proto.Uint32(1)},
			{Offset: proto.Int32(7), Length: proto.Uint32(1)},
			{Offset: proto.Int32(4), Length: proto.Uint32(1)},
		},
		PositiveDelta: []int64{1, 0, 0},
	}

	got := decodeNativeHistogram(h)

	// 1 zero bucket + 3 positive buckets, sorted by lower bound ascending.
	if len(got) != 4 {
		t.Fatalf("decodeNativeHistogram() returned %d buckets, want 4: %+v", len(got), got)
	}
	// Zero bucket first.
	if !got[0].isZero || got[0].count != 1 {
		t.Errorf("bucket[0] = %+v, want zero bucket with count 1", got[0])
	}
	// Positive buckets: index 0 -> (2^(-1/8), 1], index 8 -> (2^(7/8), 2], index 13.
	const eps = 1e-9
	wantPos := []struct{ lower, upper, count float64 }{
		{lower: math.Exp2(-1.0 / 8.0), upper: 1, count: 1},
		{lower: math.Exp2(7.0 / 8.0), upper: 2, count: 1},
		{lower: math.Exp2(12.0 / 8.0), upper: math.Exp2(13.0 / 8.0), count: 1},
	}
	for i, w := range wantPos {
		b := got[i+1]
		if math.Abs(b.lower-w.lower) > eps || math.Abs(b.upper-w.upper) > eps || b.count != w.count {
			t.Errorf("positive bucket[%d] = %+v, want {lower:%v upper:%v count:%v}", i, b, w.lower, w.upper, w.count)
		}
	}

	// Float native histogram with a negative bucket: schema 0, one negative
	// bucket at index 1 (covers value range [-2,-1)) with absolute count 5.
	hf := &dto.Histogram{
		SampleCountFloat: proto.Float64(5),
		SampleSum:        proto.Float64(-7),
		Schema:           proto.Int32(0),
		ZeroThreshold:    proto.Float64(0),
		NegativeSpan: []*dto.BucketSpan{
			{Offset: proto.Int32(1), Length: proto.Uint32(1)},
		},
		NegativeCount: []float64{5},
	}
	gotf := decodeNativeHistogram(hf)
	if len(gotf) != 1 {
		t.Fatalf("decodeNativeHistogram(float) returned %d buckets, want 1: %+v", len(gotf), gotf)
	}
	nb := gotf[0]
	if math.Abs(nb.lower+2) > eps || math.Abs(nb.upper+1) > eps || nb.count != 5 {
		t.Errorf("negative bucket = %+v, want {lower:-2 upper:-1 count:5}", nb)
	}
}
