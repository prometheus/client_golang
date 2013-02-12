// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package maths

import (
	"math"
)

// Go's standard library does not offer a factorial function.
func Factorial(of int) int64 {
	if of <= 0 {
		return 1
	}

	var result int64 = 1

	for i := int64(of); i >= 1; i-- {
		result *= i
	}

	return result
}

// Calculate the value of a probability density for a given binomial statistic,
// where k is the target count of true cases, n is the number of subjects, and
// p is the probability.
func BinomialPDF(k, n int, p float64) float64 {
	binomialCoefficient := float64(Factorial(n)) / float64(Factorial(k)*Factorial(n-k))
	intermediate := math.Pow(p, float64(k)) * math.Pow(1-p, float64(n-k))

	return binomialCoefficient * intermediate
}
