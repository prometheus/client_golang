// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"math"
	"sort"
)

// TODO(mtp): Split this out into a summary statistics file once moving/rolling
//            averages are calculated.

// ReductionMethod provides a method for reducing metrics into a scalar value.
type ReductionMethod func([]float64) float64

var (
	medianReducer = NearestRankReducer(50)
)

// These are the canned ReductionMethods.
var (
	// Reduce to the average of the set.
	AverageReducer = averageReducer

	// Extract the first modal value.
	FirstModeReducer = firstModeReducer

	// Reduce to the maximum of the set.
	MaximumReducer = maximumReducer

	// Reduce to the median of the set.
	MedianReducer = medianReducer

	// Reduce to the minimum of the set.
	MinimumReducer = minimumReducer
)

func averageReducer(input []float64) float64 {
	count := 0.0
	sum := 0.0

	for _, v := range input {
		sum += v
		count++
	}

	if count == 0 {
		return math.NaN()
	}

	return sum / count
}

func firstModeReducer(input []float64) float64 {
	valuesToFrequency := map[float64]int64{}
	largestTally := int64(math.MinInt64)
	largestTallyValue := math.NaN()

	for _, v := range input {
		presentCount, _ := valuesToFrequency[v]
		presentCount++

		valuesToFrequency[v] = presentCount

		if presentCount > largestTally {
			largestTally = presentCount
			largestTallyValue = v
		}
	}

	return largestTallyValue
}

// Calculate the percentile by choosing the nearest neighboring value.
func nearestRank(input []float64, percentile float64) float64 {
	inputSize := len(input)

	if inputSize == 0 {
		return math.NaN()
	}

	ordinalRank := math.Ceil(((percentile / 100.0) * float64(inputSize)) + 0.5)

	copiedInput := make([]float64, inputSize)
	copy(copiedInput, input)
	sort.Float64s(copiedInput)

	preliminaryIndex := int(ordinalRank) - 1

	if preliminaryIndex == inputSize {
		return copiedInput[preliminaryIndex-1]
	}

	return copiedInput[preliminaryIndex]
}

// Generate a ReductionMethod based off of extracting a given percentile value.
func NearestRankReducer(percentile float64) ReductionMethod {
	return func(input []float64) float64 {
		return nearestRank(input, percentile)
	}
}

func minimumReducer(input []float64) float64 {
	minimum := math.MaxFloat64

	for _, v := range input {
		minimum = math.Min(minimum, v)
	}

	return minimum
}

func maximumReducer(input []float64) float64 {
	maximum := math.SmallestNonzeroFloat64

	for _, v := range input {
		maximum = math.Max(maximum, v)
	}

	return maximum
}
