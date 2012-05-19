// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// tallying_bucket.go provides a histogram bucket type that aggregates tallies
// of events that fall into its ranges versus a summary of the values
// themselves.

package metrics

import (
	"fmt"
	"github.com/matttproud/golang_instrumentation/maths"
	"math"
	"sync"
)

const (
	lowerThird = 100.0 / 3.0
	upperThird = 2.0 * (100.0 / 3.0)
)

// A TallyingIndexEstimator is responsible for estimating the value of index for
// a given TallyingBucket, even though a TallyingBucket does not possess a
// collection of samples.  There are a few strategies listed below for how
// this value should be approximated.
type TallyingIndexEstimator func(minimum, maximum float64, index, observations int) float64

// Provide a filter for handling empty buckets.
func emptyFilter(e TallyingIndexEstimator) TallyingIndexEstimator {
	return func(minimum, maximum float64, index, observations int) float64 {
		if observations == 0 {
			return math.NaN()
		}

		return e(minimum, maximum, index, observations)
	}
}

// Report the smallest observed value in the bucket.
var Minimum TallyingIndexEstimator = emptyFilter(func(minimum, maximum float64, _, observations int) float64 {
	return minimum
})

// Report the largest observed value in the bucket.
var Maximum TallyingIndexEstimator = emptyFilter(func(minimum, maximum float64, _, observations int) float64 {
	return maximum
})

// Report the average of the extrema.
var Average TallyingIndexEstimator = emptyFilter(func(minimum, maximum float64, _, observations int) float64 {
	return maths.Average([]float64{minimum, maximum})
})

// Report the minimum value of the index is in the lower-third of observations,
// the average if in the middle-third, and the maximum if in the largest third.
var Uniform TallyingIndexEstimator = emptyFilter(func(minimum, maximum float64, index, observations int) float64 {
	if observations == 1 {
		return minimum
	}

	location := float64(index) / float64(observations)

	if location > upperThird {
		return maximum
	} else if location < lowerThird {
		return minimum
	}

	return maths.Average([]float64{minimum, maximum})
})

// A TallyingBucket is a Bucket that tallies when an object is added to it.
// Upon insertion, an object is compared against collected extrema and noted
// as a new minimum or maximum if appropriate.
type TallyingBucket struct {
	observations     int
	smallestObserved float64
	largestObserved  float64
	mutex            sync.RWMutex
	estimator        TallyingIndexEstimator
}

func (b *TallyingBucket) Add(value float64) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.observations += 1
	b.smallestObserved = math.Min(value, b.smallestObserved)
	b.largestObserved = math.Max(value, b.largestObserved)
}

func (b *TallyingBucket) Humanize() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	observations := b.observations

	if observations == 0 {
		return fmt.Sprintf("[TallyingBucket (Empty)]")
	}

	return fmt.Sprintf("[TallyingBucket (%f, %f); %d items]", b.smallestObserved, b.largestObserved, observations)
}

func (b *TallyingBucket) Observations() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.observations
}

func (b *TallyingBucket) ValueForIndex(index int) float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.estimator(b.smallestObserved, b.largestObserved, index, b.observations)
}

// Produce a TallyingBucket with sane defaults.
func DefaultTallyingBucket() TallyingBucket {
	return TallyingBucket{
		smallestObserved: math.MaxFloat64,
		largestObserved:  math.SmallestNonzeroFloat64,
		estimator:        Minimum,
	}
}

func CustomTallyingBucket(estimator TallyingIndexEstimator) TallyingBucket {
	return TallyingBucket{
		smallestObserved: math.MaxFloat64,
		largestObserved:  math.SmallestNonzeroFloat64,
		estimator:        estimator,
	}
}

// This is used strictly for testing.
func TallyingBucketBuilder() Bucket {
	b := DefaultTallyingBucket()
	return &b
}
