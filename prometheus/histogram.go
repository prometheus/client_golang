// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"

	dto "github.com/prometheus/client_model/go"

	"code.google.com/p/goprotobuf/proto"
)

// This generates count-buckets of equal size distributed along the open
// interval of lower to upper.  For instance, {lower=0, upper=10, count=5}
// yields the following: [0, 2, 4, 6, 8].
func EquallySizedBucketsFor(lower, upper float64, count int) []float64 {
	buckets := make([]float64, count)

	partitionSize := (upper - lower) / float64(count)

	for i := 0; i < count; i++ {
		m := float64(i)
		buckets[i] = lower + (m * partitionSize)
	}

	return buckets
}

// This generates log2-sized buckets spanning from lower to upper inclusively
// as well as values beyond it.
func LogarithmicSizedBucketsFor(lower, upper float64) []float64 {
	bucketCount := int(math.Ceil(math.Log2(upper)))

	buckets := make([]float64, bucketCount)

	for i, j := 0, 0.0; i < bucketCount; i, j = i+1, math.Pow(2, float64(i+1.0)) {
		buckets[i] = j
	}

	return buckets
}

// A HistogramSpecification defines how a Histogram is to be built.
type HistogramSpecification struct {
	BucketBuilder         BucketBuilder
	ReportablePercentiles []float64
	Starts                []float64
}

type Histogram interface {
	Metric
	Add(labels map[string]string, value float64)
}

// The histogram is an accumulator for samples.  It merely routes into which
// bucket to capture an event and provides a percentile calculation mechanism.
type histogram struct {
	bucketMaker BucketBuilder
	// This represents the open interval's start at which values shall be added to
	// the bucket.  The interval continues until the beginning of the next bucket
	// exclusive or positive infinity.
	//
	// N.B.
	// - bucketStarts should be sorted in ascending order;
	// - len(bucketStarts) must be equivalent to len(buckets);
	// - The index of a given bucketStarts' element is presumed to
	//   correspond to the appropriate element in buckets.
	bucketStarts []float64
	mutex        sync.RWMutex
	// These are the buckets that capture samples as they are emitted to the
	// histogram.  Please consult the reference interface and its implements for
	// further details about behavior expectations.
	values map[uint64]*histogramVector
	// These are the percentile values that will be reported on marshalling.
	reportablePercentiles []float64
}

type histogramVector struct {
	buckets []Bucket
	labels  map[string]string
	sum     float64
	count   uint64
}

func (h *histogram) Add(labels map[string]string, value float64) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if labels == nil {
		labels = map[string]string{}
	}

	signature := labelsToSignature(labels)
	var histogram *histogramVector = nil
	if original, ok := h.values[signature]; ok {
		histogram = original
	} else {
		bucketCount := len(h.bucketStarts)
		histogram = &histogramVector{
			buckets: make([]Bucket, bucketCount),
			labels:  labels,
		}
		for i := 0; i < bucketCount; i++ {
			histogram.buckets[i] = h.bucketMaker()
		}
		h.values[signature] = histogram
	}

	lastIndex := 0

	for i, bucketStart := range h.bucketStarts {
		if value < bucketStart {
			break
		}

		lastIndex = i
	}

	histogram.buckets[lastIndex].Add(value)

	histogram.sum += value
	histogram.count++
}

func (h *histogram) String() string {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	stringBuffer := &bytes.Buffer{}
	stringBuffer.WriteString("[Histogram { ")

	for _, histogram := range h.values {
		fmt.Fprintf(stringBuffer, "Labels: %s ", histogram.labels)
		for i, bucketStart := range h.bucketStarts {
			bucket := histogram.buckets[i]
			fmt.Fprintf(stringBuffer, "[%f, inf) = %s, ", bucketStart, bucket)
		}
	}

	stringBuffer.WriteString("}]")

	return stringBuffer.String()
}

// Determine the number of previous observations up to a given index.
func previousCumulativeObservations(cumulativeObservations []int, bucketIndex int) int {
	if bucketIndex == 0 {
		return 0
	}

	return cumulativeObservations[bucketIndex-1]
}

// Determine the index for an element given a percentage of length.
func prospectiveIndexForPercentile(percentile float64, totalObservations int) int {
	return int(percentile * float64(totalObservations-1))
}

// Determine the next bucket element when interim bucket intervals may be empty.
func (h histogram) nextNonEmptyBucketElement(signature uint64, currentIndex, bucketCount int, observationsByBucket []int) (*Bucket, int) {
	for i := currentIndex; i < bucketCount; i++ {
		if observationsByBucket[i] == 0 {
			continue
		}

		histogram := h.values[signature]

		return &histogram.buckets[i], 0
	}

	panic("Illegal Condition: There were no remaining buckets to provide a value.")
}

// Find what bucket and element index contains a given percentile value.
// If a percentile is requested that results in a corresponding index that is no
// longer contained by the bucket, the index of the last item is returned.  This
// may occur if the underlying bucket catalogs values and employs an eviction
// strategy.
func (h histogram) bucketForPercentile(signature uint64, percentile float64) (*Bucket, int) {
	bucketCount := len(h.bucketStarts)

	// This captures the quantity of samples in a given bucket's range.
	observationsByBucket := make([]int, bucketCount)
	// This captures the cumulative quantity of observations from all preceding
	// buckets up and to the end of this bucket.
	cumulativeObservationsByBucket := make([]int, bucketCount)

	totalObservations := 0

	histogram := h.values[signature]

	for i, bucket := range histogram.buckets {
		observations := bucket.Observations()
		observationsByBucket[i] = observations
		totalObservations += bucket.Observations()
		cumulativeObservationsByBucket[i] = totalObservations
	}

	// This captures the index offset where the given percentile value would be
	// were all submitted samples stored and never down-/re-sampled nor deleted
	// and housed in a singular array.
	prospectiveIndex := prospectiveIndexForPercentile(percentile, totalObservations)

	for i, cumulativeObservation := range cumulativeObservationsByBucket {
		if cumulativeObservation == 0 {
			continue
		}

		// Find the bucket that contains the given index.
		if cumulativeObservation >= prospectiveIndex {
			var subIndex int
			// This calculates the index within the current bucket where the given
			// percentile may be found.
			subIndex = prospectiveIndex - previousCumulativeObservations(cumulativeObservationsByBucket, i)

			// Sometimes the index may be the last item, in which case we need to
			// take this into account.
			if observationsByBucket[i] == subIndex {
				return h.nextNonEmptyBucketElement(signature, i+1, bucketCount, observationsByBucket)
			}

			return &histogram.buckets[i], subIndex
		}
	}

	return &histogram.buckets[0], 0
}

// Return the histogram's estimate of the value for a given percentile of
// collected samples.  The requested percentile is expected to be a real
// value within (0, 1.0].
func (h histogram) percentile(signature uint64, percentile float64) float64 {
	bucket, index := h.bucketForPercentile(signature, percentile)

	return (*bucket).ValueForIndex(index)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, floatFormat, floatPrecision, floatBitCount)
}

func (h *histogram) MarshalJSON() ([]byte, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	values := make([]map[string]interface{}, 0, len(h.values))

	for signature, value := range h.values {
		percentiles := make(map[string]float64, len(h.reportablePercentiles))

		for _, percentile := range h.reportablePercentiles {
			formatted := formatFloat(percentile)
			percentiles[formatted] = h.percentile(signature, percentile)
		}

		values = append(values, map[string]interface{}{
			labelsKey: value.labels,
			valueKey:  percentiles,
		})
	}

	return json.Marshal(map[string]interface{}{
		typeKey:  histogramTypeValue,
		valueKey: values,
	})
}

func (h *histogram) ResetAll() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	for signature, value := range h.values {
		for _, bucket := range value.buckets {
			bucket.Reset()
		}

		delete(h.values, signature)
	}
}

// Produce a histogram from a given specification.
func NewHistogram(specification *HistogramSpecification) Histogram {
	metric := &histogram{
		bucketMaker:           specification.BucketBuilder,
		bucketStarts:          specification.Starts,
		reportablePercentiles: specification.ReportablePercentiles,
		values:                map[uint64]*histogramVector{},
	}

	return metric
}

// Furnish a Histogram with unsensible default values and behaviors that is
// strictly useful for prototyping purposes.
func NewDefaultHistogram() Histogram {
	return NewHistogram(
		&HistogramSpecification{
			Starts:                LogarithmicSizedBucketsFor(0, 4096),
			BucketBuilder:         AccumulatingBucketBuilder(EvictAndReplaceWith(10, AverageReducer), 50),
			ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.90, 0.99},
		},
	)
}

func (metric *histogram) dumpChildren(f *dto.MetricFamily) {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	f.Type = dto.MetricType_SUMMARY.Enum()

	for signature, child := range metric.values {
		c := &dto.Summary{
			SampleSum:   proto.Float64(child.sum),
			SampleCount: proto.Uint64(child.count),
		}

		m := &dto.Metric{
			Summary: c,
		}

		for name, value := range child.labels {
			p := &dto.LabelPair{
				Name:  proto.String(name),
				Value: proto.String(value),
			}

			m.Label = append(m.Label, p)
		}

		for _, percentile := range metric.reportablePercentiles {
			q := &dto.Quantile{
				Quantile: proto.Float64(percentile),
				Value:    proto.Float64(metric.percentile(signature, percentile)),
			}

			c.Quantile = append(c.Quantile, q)
		}

		f.Metric = append(f.Metric, m)
	}
}
