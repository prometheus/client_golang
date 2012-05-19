// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// histogram.go provides a basic histogram metric, which can accumulate scalar
// event values or samples.  The underlying histogram implementation is designed
// to be performant in that it accepts tolerable inaccuracies.

// TOOD(mtp): Implement visualization and exporting.

package metrics

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
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
	Starts                []float64
	BucketMaker           BucketBuilder
	ReportablePercentiles []float64
}

// The histogram is an accumulator for samples.  It merely routes into which
// to bucket to capture an event and provides a percentile calculation
// mechanism.
//
// Histogram makes do without locking by employing the law of large numbers
// to presume a convergence toward a given bucket distribution.  Locking
// may be implemented in the buckets themselves, though.
type Histogram struct {
	// This represents the open interval's start at which values shall be added to
	// the bucket.  The interval continues until the beginning of the next bucket
	// exclusive or positive infinity.
	//
	// N.B.
	// - bucketStarts should be sorted in ascending order;
	// - len(bucketStarts) must be equivalent to len(buckets);
	// - The index of a given bucketStarts' element is presumed to match
	//   correspond to the appropriate element in buckets.
	bucketStarts []float64
	// These are the buckets that capture samples as they are emitted to the
	// histogram.  Please consult the reference interface and its implements for
	// further details about behavior expectations.
	buckets []Bucket
	// These are the percentile values that will be reported on marshalling.
	reportablePercentiles []float64
}

func (h *Histogram) Add(value float64) {
	lastIndex := 0

	for i, bucketStart := range h.bucketStarts {
		if value < bucketStart {
			break
		}

		lastIndex = i
	}

	h.buckets[lastIndex].Add(value)
}

func (h *Histogram) Humanize() string {
	stringBuffer := bytes.NewBufferString("")
	stringBuffer.WriteString("[Histogram { ")

	for i, bucketStart := range h.bucketStarts {
		bucket := h.buckets[i]
		stringBuffer.WriteString(fmt.Sprintf("[%f, inf) = %s, ", bucketStart, bucket.Humanize()))
	}

	stringBuffer.WriteString("}]")

	return string(stringBuffer.Bytes())
}

func previousCumulativeObservations(cumulativeObservations []int, bucketIndex int) int {
	if bucketIndex == 0 {
		return 0
	}

	return cumulativeObservations[bucketIndex-1]
}

func prospectiveIndexForPercentile(percentile float64, totalObservations int) int {
	return int(math.Floor(percentile * float64(totalObservations)))
}

// Find what bucket and element index contains a given percentile value.
// If a percentile is requested that results in a corresponding index that is no
// longer contained by the bucket, the index of the last item is returned.  This
// may occur if the underlying bucket catalogs values and employs an eviction
// strategy.
func (h *Histogram) bucketForPercentile(percentile float64) (bucket *Bucket, index int) {
	bucketCount := len(h.buckets)

	observationsByBucket := make([]int, bucketCount)
	cumulativeObservationsByBucket := make([]int, bucketCount)
	cumulativePercentagesByBucket := make([]float64, bucketCount)

	var totalObservations int = 0

	for i, bucket := range h.buckets {
		observations := bucket.Observations()
		observationsByBucket[i] = observations
		totalObservations += bucket.Observations()
		cumulativeObservationsByBucket[i] = totalObservations
	}

	for i, _ := range h.buckets {
		cumulativePercentagesByBucket[i] = float64(cumulativeObservationsByBucket[i]) / float64(totalObservations)
	}

	prospectiveIndex := prospectiveIndexForPercentile(percentile, totalObservations)

	for i, cumulativeObservation := range cumulativeObservationsByBucket {
		if cumulativeObservation == 0 {
			continue
		}

		if cumulativeObservation >= prospectiveIndex {
			var subIndex int
			subIndex = prospectiveIndex - previousCumulativeObservations(cumulativeObservationsByBucket, i)
			if observationsByBucket[i] == subIndex {
				subIndex--
			}

			return &h.buckets[i], subIndex
		}
	}

	return &h.buckets[0], 0
}

// Return the histogram's estimate of the value for a given percentile of
// collected samples.  The requested percentile is expected to be a real
// value within (0, 1.0].
func (h *Histogram) Percentile(percentile float64) float64 {
	bucket, index := h.bucketForPercentile(percentile)

	return (*bucket).ValueForIndex(index)
}

func (h *Histogram) Marshallable() map[string]interface{} {
	numberOfPercentiles := len(h.reportablePercentiles)
	result := make(map[string]interface{}, 2)

	result["type"] = "histogram"

	value := make(map[string]interface{}, numberOfPercentiles)

	for _, percentile := range h.reportablePercentiles {
		percentileString := strconv.FormatFloat(percentile, 'f', 6, 64)
		value[percentileString] = strconv.FormatFloat(h.Percentile(percentile), 'f', 6, 64)
	}

	result["value"] = value

	return result
}

// Produce a histogram from a given specification.
func CreateHistogram(specification *HistogramSpecification) *Histogram {
	bucketCount := len(specification.Starts)

	metric := &Histogram{
		bucketStarts:          specification.Starts,
		buckets:               make([]Bucket, bucketCount),
		reportablePercentiles: specification.ReportablePercentiles,
	}

	for i := 0; i < bucketCount; i++ {
		metric.buckets[i] = specification.BucketMaker()
	}

	return metric
}
