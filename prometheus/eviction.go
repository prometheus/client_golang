// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"container/heap"
	"time"
)

// EvictionPolicy implements some sort of garbage collection methodology for
// an underlying heap.Interface.  This is presently only used for
// AccumulatingBucket.
type EvictionPolicy func(h heap.Interface)

// As the name implies, this evicts the oldest x objects from the heap.
func EvictOldest(count int) EvictionPolicy {
	return func(h heap.Interface) {
		for i := 0; i < count; i++ {
			heap.Pop(h)
		}
	}
}

// This factory produces an EvictionPolicy that applies some standardized
// reduction methodology on the to-be-terminated values.
func EvictAndReplaceWith(count int, reducer ReductionMethod) EvictionPolicy {
	return func(h heap.Interface) {
		oldValues := make([]float64, count)

		for i := 0; i < count; i++ {
			oldValues[i] = heap.Pop(h).(*item).Value.(float64)
		}

		reduced := reducer(oldValues)

		heap.Push(h, &item{
			Value: reduced,
			//	TODO(mtp): Parameterize the priority generation since these tools are
			//             useful.
			Priority: -1 * time.Now().UnixNano(),
		})
	}
}
