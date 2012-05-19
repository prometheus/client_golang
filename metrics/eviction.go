// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// eviction.go provides several histogram bucket eviction strategies.

package metrics

import (
	"container/heap"
	"github.com/matttproud/golang_instrumentation/maths"
	"github.com/matttproud/golang_instrumentation/utility"
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
//
// TODO(mtp): Parameterize the priority generation since these tools are useful.
func EvictAndReplaceWith(count int, reducer maths.ReductionMethod) EvictionPolicy {
	return func(h heap.Interface) {
		oldValues := make([]float64, count)

		for i := 0; i < count; i++ {
			oldValues[i] = heap.Pop(h).(*utility.Item).Value.(float64)
		}

		reduced := reducer(oldValues)

		heap.Push(h, &utility.Item{
			Value:    reduced,
			Priority: -1 * time.Now().UnixNano(),
		})
	}
}
