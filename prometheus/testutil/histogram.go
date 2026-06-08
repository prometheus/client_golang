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
	"fmt"
	"math"
	"sort"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

// GatherAndAssertNativeHistogramExists gathers all metrics from the provided
// Gatherer and returns nil if a native histogram with the given name (and, if
// labels are provided, matching those labels) was gathered. It returns an error
// otherwise, including when the named metric is a classic (non-native) histogram.
func GatherAndAssertNativeHistogramExists(g prometheus.Gatherer, name string, labels prometheus.Labels) error {
	_, err := findNativeHistogram(g, name, labels)
	return err
}

// GatherAndAssertNativeHistogramCount gathers all metrics from the provided
// Gatherer and asserts that the native histogram identified by name and labels
// has the expected number of observations. The total observation count is exact,
// so there is no tolerance (unlike GatherAndAssertNativeHistogramIntervalCount,
// which estimates).
func GatherAndAssertNativeHistogramCount(g prometheus.Gatherer, name string, labels prometheus.Labels, want uint64) error {
	h, err := findNativeHistogram(g, name, labels)
	if err != nil {
		return err
	}
	got := nativeHistogramCount(h)
	if got != float64(want) {
		return fmt.Errorf("unexpected observation count for native histogram %q: got %v, want %d", name, got, want)
	}
	return nil
}

// GatherAndAssertNativeHistogramSum gathers all metrics from the provided
// Gatherer and asserts that the sum of observations of the native histogram
// identified by name and labels equals want within the given tolerance.
func GatherAndAssertNativeHistogramSum(g prometheus.Gatherer, name string, labels prometheus.Labels, want, tolerance float64) error {
	h, err := findNativeHistogram(g, name, labels)
	if err != nil {
		return err
	}
	got := h.GetSampleSum()
	if math.Abs(got-want) > tolerance {
		return fmt.Errorf("unexpected sum for native histogram %q: got %v, want %v (tolerance %v)", name, got, want, tolerance)
	}
	return nil
}

// GatherAndAssertNativeHistogramIntervalCount gathers all metrics from the
// provided Gatherer and asserts that the estimated number of observations of the
// native histogram identified by name and labels that fall into the half-open
// interval (lower, upper] equals want within the given tolerance. The lower
// bound is exclusive and the upper bound is inclusive, matching the semantics of
// Prometheus' histogram_fraction function; an observation sitting exactly on
// lower therefore falls into the preceding interval and is not counted, while one
// sitting exactly on upper is.
//
// Because native histogram buckets are exponential, the count is an estimate: a
// bucket that straddles a boundary is interpolated (logarithmically for regular
// buckets, linearly for the zero bucket), again matching histogram_fraction.
// Choose lower and upper to coincide with bucket boundaries for an exact result.
func GatherAndAssertNativeHistogramIntervalCount(g prometheus.Gatherer, name string, labels prometheus.Labels, lower, upper, want, tolerance float64) error {
	h, err := findNativeHistogram(g, name, labels)
	if err != nil {
		return err
	}
	buckets := decodeNativeHistogram(h)
	got := nativeHistogramRankAt(buckets, upper) - nativeHistogramRankAt(buckets, lower)
	if math.Abs(got-want) > tolerance {
		return fmt.Errorf(
			"unexpected observation count in interval (%v, %v] for native histogram %q: got %v, want %v (tolerance %v)",
			lower, upper, name, got, want, tolerance,
		)
	}
	return nil
}

// findNativeHistogram gathers metrics from g, locates the metric family named
// name, selects the single series matching labels, and returns its histogram if
// it is a native histogram. It returns a descriptive error otherwise.
func findNativeHistogram(g prometheus.Gatherer, name string, labels prometheus.Labels) (*dto.Histogram, error) {
	mfs, err := g.Gather()
	if err != nil {
		return nil, fmt.Errorf("gathering metrics failed: %w", err)
	}

	var mf *dto.MetricFamily
	for _, candidate := range mfs {
		if candidate.GetName() == name {
			mf = candidate
			break
		}
	}
	if mf == nil {
		return nil, fmt.Errorf("no metric family named %q was gathered", name)
	}

	var matches []*dto.Metric
	for _, m := range mf.GetMetric() {
		if metricMatchesLabels(m, labels) {
			matches = append(matches, m)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no series of metric %q matched the labels %v", name, labels)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("%d series of metric %q matched the labels %v, expected exactly one", len(matches), name, labels)
	}

	h := matches[0].GetHistogram()
	if h == nil {
		return nil, fmt.Errorf("metric %q is not a histogram", name)
	}
	if !isNativeHistogram(h) {
		return nil, fmt.Errorf("metric %q is not a native histogram", name)
	}
	return h, nil
}

// isNativeHistogram reports whether h carries native histogram data. The Schema
// pointer is the primary signal: client_golang only populates the native fields
// (Schema included) when sparse buckets are enabled, so a purely classic
// histogram has a nil Schema.
func isNativeHistogram(h *dto.Histogram) bool {
	return h.Schema != nil || len(h.GetPositiveSpan()) > 0 || len(h.GetNegativeSpan()) > 0
}

// metricMatchesLabels reports whether m carries every label in labels with the
// given value. An empty selector matches any metric.
func metricMatchesLabels(m *dto.Metric, labels prometheus.Labels) bool {
	for labelName, labelValue := range labels {
		matched := false
		for _, lp := range m.GetLabel() {
			if lp.GetName() == labelName && lp.GetValue() == labelValue {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// nativeHistogramCount returns the number of observations recorded in h. The
// float count overrides the integer count when greater than zero, as defined by
// the SampleCountFloat field for float (gauge) histograms.
func nativeHistogramCount(h *dto.Histogram) float64 {
	if h.GetSampleCountFloat() > 0 {
		return h.GetSampleCountFloat()
	}
	return float64(h.GetSampleCount())
}

// nativeHistogramRankAt returns the estimated cumulative number of observations
// with a value less than or equal to v, given buckets sorted ascending by lower
// bound. A bucket that straddles v contributes an interpolated fraction of its
// count.
func nativeHistogramRankAt(buckets []nativeHistogramBucket, v float64) float64 {
	var rank float64
	for _, b := range buckets {
		switch {
		case b.upper <= v:
			rank += b.count
		case b.lower >= v:
			return rank
		default:
			rank += b.count * nativeHistogramBucketFraction(b, v)
			return rank
		}
	}
	return rank
}

// nativeHistogramBucketFraction returns the fraction of bucket b that lies at or
// below v, with lower < v < upper. The zero bucket is interpolated linearly
// (uniform in value); regular exponential buckets are interpolated on a log2
// scale (uniform in log of magnitude), matching Prometheus' histogram_fraction.
func nativeHistogramBucketFraction(b nativeHistogramBucket, v float64) float64 {
	if b.isZero {
		if b.upper == b.lower {
			return 0
		}
		return (v - b.lower) / (b.upper - b.lower)
	}
	if b.lower > 0 {
		// Positive exponential bucket.
		logLower, logUpper := math.Log2(b.lower), math.Log2(b.upper)
		if logUpper == logLower {
			return 0
		}
		return (math.Log2(v) - logLower) / (logUpper - logLower)
	}
	// Negative exponential bucket: interpolate on the magnitude. As v increases
	// toward zero its magnitude shrinks, so the fraction below v grows with the
	// magnitude of the lower bound.
	logLower, logUpper := math.Log2(-b.lower), math.Log2(-b.upper)
	if logLower == logUpper {
		return 0
	}
	return (logLower - math.Log2(-v)) / (logLower - logUpper)
}

// nativeHistogramBucket is a single reconstructed bucket of a native histogram.
// For positive buckets the covered range is (lower, upper]; for negative
// buckets it is [lower, upper); for the zero bucket it is [lower, upper] with
// lower <= 0 <= upper.
type nativeHistogramBucket struct {
	lower, upper float64
	count        float64
	isZero       bool
}

// nativeHistogramUpperBound returns the upper bound of the bucket with the given
// index for the given (standard exponential) schema, i.e. 2^(index * 2^-schema).
func nativeHistogramUpperBound(schema int32, index int) float64 {
	if schema <= 0 {
		// base = 2^(2^-schema) is an integer power of two, so the bound is an
		// exact power of two: 2^(index << -schema). math.Ldexp keeps it exact.
		return math.Ldexp(1, index<<uint(-schema))
	}
	return math.Exp2(float64(index) * math.Exp2(-float64(schema)))
}

// decodeSpanIndices expands a list of bucket spans into the absolute bucket
// indices they represent. The offset of the first span is the absolute index of
// its first bucket (and may be negative); the offset of every subsequent span is
// the number of empty buckets between it and the previous span.
func decodeSpanIndices(spans []*dto.BucketSpan) []int {
	var indices []int
	index := 0
	for _, span := range spans {
		index += int(span.GetOffset())
		for range span.GetLength() {
			indices = append(indices, index)
			index++
		}
	}
	return indices
}

// decodeBucketCounts returns the absolute per-bucket counts. Integer native
// histograms carry deltas (the first element is absolute, the rest are deltas
// relative to the previous bucket); float native histograms carry absolute
// counts directly. counts takes precedence when present.
func decodeBucketCounts(deltas []int64, counts []float64) []float64 {
	if len(counts) > 0 {
		out := make([]float64, len(counts))
		copy(out, counts)
		return out
	}
	out := make([]float64, len(deltas))
	var cumulative int64
	for i, d := range deltas {
		cumulative += d
		out[i] = float64(cumulative)
	}
	return out
}

// decodeNativeHistogram reconstructs the populated buckets of a native histogram
// (zero, positive, and negative), sorted in ascending order of their lower bound.
func decodeNativeHistogram(h *dto.Histogram) []nativeHistogramBucket {
	schema := h.GetSchema()
	var buckets []nativeHistogramBucket

	zeroCount := float64(h.GetZeroCount())
	if h.GetZeroCountFloat() > 0 {
		zeroCount = h.GetZeroCountFloat()
	}
	zeroThreshold := h.GetZeroThreshold()
	if zeroCount > 0 || zeroThreshold > 0 {
		buckets = append(buckets, nativeHistogramBucket{
			lower:  -zeroThreshold,
			upper:  zeroThreshold,
			count:  zeroCount,
			isZero: true,
		})
	}

	posCounts := decodeBucketCounts(h.GetPositiveDelta(), h.GetPositiveCount())
	for i, index := range decodeSpanIndices(h.GetPositiveSpan()) {
		buckets = append(buckets, nativeHistogramBucket{
			lower: nativeHistogramUpperBound(schema, index-1),
			upper: nativeHistogramUpperBound(schema, index),
			count: posCounts[i],
		})
	}

	negCounts := decodeBucketCounts(h.GetNegativeDelta(), h.GetNegativeCount())
	for i, index := range decodeSpanIndices(h.GetNegativeSpan()) {
		// A negative bucket with index i covers the value range
		// [-upper(i), -upper(i-1)).
		buckets = append(buckets, nativeHistogramBucket{
			lower: -nativeHistogramUpperBound(schema, index),
			upper: -nativeHistogramUpperBound(schema, index-1),
			count: negCounts[i],
		})
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].lower < buckets[j].lower
	})
	return buckets
}
