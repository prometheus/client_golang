// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maths

import (
	"math"
	"sort"
)

// TODO(mtp): Split this out into a summary statistics file once moving/rolling
//            averages are calculated.

// ReductionMethod provides a method for reducing metrics into a given scalar
// value.
type ReductionMethod func([]float64) float64

var Average ReductionMethod = func(input []float64) float64 {
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

// Extract the first modal value.
var FirstMode ReductionMethod = func(input []float64) float64 {
	valuesToFrequency := map[float64]int64{}
	var largestTally int64 = math.MinInt64
	var largestTallyValue float64 = math.NaN()

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
func NearestRank(input []float64, percentile float64) float64 {
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
		return NearestRank(input, percentile)
	}
}

var Median ReductionMethod = NearestRankReducer(50)

var Minimum ReductionMethod = func(input []float64) float64 {
	var minimum float64 = math.MaxFloat64

	for _, v := range input {
		minimum = math.Min(minimum, v)
	}

	return minimum
}

var Maximum ReductionMethod = func(input []float64) float64 {
	var maximum float64 = math.SmallestNonzeroFloat64

	for _, v := range input {
		maximum = math.Max(maximum, v)
	}

	return maximum
}
