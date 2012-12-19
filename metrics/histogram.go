/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package metrics

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
)

/*
This generates count-buckets of equal size distributed along the open
interval of lower to upper.  For instance, {lower=0, upper=10, count=5}
yields the following: [0, 2, 4, 6, 8].
*/
func EquallySizedBucketsFor(lower, upper float64, count int) []float64 {
	buckets := make([]float64, count)

	partitionSize := (upper - lower) / float64(count)

	for i := 0; i < count; i++ {
		m := float64(i)
		buckets[i] = lower + (m * partitionSize)
	}

	return buckets
}

/*
This generates log2-sized buckets spanning from lower to upper inclusively
as well as values beyond it.
*/
func LogarithmicSizedBucketsFor(lower, upper float64) []float64 {
	bucketCount := int(math.Ceil(math.Log2(upper)))

	buckets := make([]float64, bucketCount)

	for i, j := 0, 0.0; i < bucketCount; i, j = i+1, math.Pow(2, float64(i+1.0)) {
		buckets[i] = j
	}

	return buckets
}

/*
A HistogramSpecification defines how a Histogram is to be built.
*/
type HistogramSpecification struct {
	Starts                []float64
	BucketMaker           BucketBuilder
	ReportablePercentiles []float64
}

/*
The histogram is an accumulator for samples.  It merely routes into which
to bucket to capture an event and provides a percentile calculation
mechanism.

Histogram makes do without locking by employing the law of large numbers
to presume a convergence toward a given bucket distribution.  Locking
may be implemented in the buckets themselves, though.
*/
type Histogram struct {
	/*
		This represents the open interval's start at which values shall be added to
		the bucket.  The interval continues until the beginning of the next bucket
		exclusive or positive infinity.

		N.B.
		- bucketStarts should be sorted in ascending order;
		- len(bucketStarts) must be equivalent to len(buckets);
		- The index of a given bucketStarts' element is presumed to match
		  correspond to the appropriate element in buckets.
	*/
	bucketStarts []float64
	/*
		These are the buckets that capture samples as they are emitted to the
		histogram.  Please consult the reference interface and its implements for
		further details about behavior expectations.
	*/
	buckets []Bucket
	/*
	 These are the percentile values that will be reported on marshalling.
	*/
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

func (h *Histogram) String() string {
	stringBuffer := bytes.NewBufferString("")
	stringBuffer.WriteString("[Histogram { ")

	for i, bucketStart := range h.bucketStarts {
		bucket := h.buckets[i]
		stringBuffer.WriteString(fmt.Sprintf("[%f, inf) = %s, ", bucketStart, bucket.String()))
	}

	stringBuffer.WriteString("}]")

	return string(stringBuffer.Bytes())
}

/*
Determine the number of previous observations up to a given index.
*/
func previousCumulativeObservations(cumulativeObservations []int, bucketIndex int) int {
	if bucketIndex == 0 {
		return 0
	}

	return cumulativeObservations[bucketIndex-1]
}

/*
Determine the index for an element given a percentage of length.
*/
func prospectiveIndexForPercentile(percentile float64, totalObservations int) int {
	return int(percentile * float64(totalObservations-1))
}

/*
Determine the next bucket element when interim bucket intervals may be empty.
*/
func (h *Histogram) nextNonEmptyBucketElement(currentIndex, bucketCount int, observationsByBucket []int) (*Bucket, int) {
	for i := currentIndex; i < bucketCount; i++ {
		if observationsByBucket[i] == 0 {
			continue
		}

		return &h.buckets[i], 0
	}

	panic("Illegal Condition: There were no remaining buckets to provide a value.")
}

/*
Find what bucket and element index contains a given percentile value.
If a percentile is requested that results in a corresponding index that is no
longer contained by the bucket, the index of the last item is returned.  This
may occur if the underlying bucket catalogs values and employs an eviction
strategy.
*/
func (h *Histogram) bucketForPercentile(percentile float64) (*Bucket, int) {
	bucketCount := len(h.buckets)

	/*
		This captures the quantity of samples in a given bucket's range.
	*/
	observationsByBucket := make([]int, bucketCount)
	/*
		This captures the cumulative quantity of observations from all preceding
		buckets up and to the end of this bucket.
	*/
	cumulativeObservationsByBucket := make([]int, bucketCount)

	var totalObservations int = 0

	for i, bucket := range h.buckets {
		observations := bucket.Observations()
		observationsByBucket[i] = observations
		totalObservations += bucket.Observations()
		cumulativeObservationsByBucket[i] = totalObservations
	}

	/*
		This captures the index offset where the given percentile value would be
		were all submitted samples stored and never down-/re-sampled nor deleted
		and housed in a singular array.
	*/
	prospectiveIndex := prospectiveIndexForPercentile(percentile, totalObservations)

	for i, cumulativeObservation := range cumulativeObservationsByBucket {
		if cumulativeObservation == 0 {
			continue
		}

		/*
			Find the bucket that contains the given index.
		*/
		if cumulativeObservation >= prospectiveIndex {
			var subIndex int
			/*
				This calculates the index within the current bucket where the given
				percentile may be found.
			*/
			subIndex = prospectiveIndex - previousCumulativeObservations(cumulativeObservationsByBucket, i)

			/*
				Sometimes the index may be the last item, in which case we need to
				take this into account.
			*/
			if observationsByBucket[i] == subIndex {
				return h.nextNonEmptyBucketElement(i+1, bucketCount, observationsByBucket)
			}

			return &h.buckets[i], subIndex
		}
	}

	return &h.buckets[0], 0
}

/*
Return the histogram's estimate of the value for a given percentile of
collected samples.  The requested percentile is expected to be a real
value within (0, 1.0].
*/
func (h *Histogram) Percentile(percentile float64) float64 {
	bucket, index := h.bucketForPercentile(percentile)

	return (*bucket).ValueForIndex(index)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, floatFormat, floatPrecision, floatBitCount)
}

func (h *Histogram) Marshallable() map[string]interface{} {
	numberOfPercentiles := len(h.reportablePercentiles)
	result := make(map[string]interface{}, 2)

	result[typeKey] = histogramTypeValue

	value := make(map[string]interface{}, numberOfPercentiles)

	for _, percentile := range h.reportablePercentiles {
		percentileString := formatFloat(percentile)
		value[percentileString] = formatFloat(h.Percentile(percentile))
	}

	result[valueKey] = value

	return result
}

/*
Produce a histogram from a given specification.
*/
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
