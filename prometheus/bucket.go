// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

// The Histogram class and associated types build buckets on their own.
type BucketBuilder func() Bucket

// This defines the base Bucket type.  The exact behaviors of the bucket are
// at the whim of the implementor.
//
// A Bucket is used as a container by Histogram as a collection for its
// accumulated samples.
type Bucket interface {
	// Add a value to the bucket.
	Add(value float64)
	// Provide a count of observations throughout the bucket's lifetime.
	Observations() int
	// Reset is responsible for resetting this bucket back to a pristine state.
	Reset()
	// Provide a humanized representation hereof.
	String() string
	// Provide the value from the given in-memory value cache or an estimate
	// thereof for the given index.  The consumer of the bucket's data makes
	// no assumptions about the underlying storage mechanisms that the bucket
	// employs.
	ValueForIndex(index int) float64
}
