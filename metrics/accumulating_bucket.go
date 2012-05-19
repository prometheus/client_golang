// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// accumulating_bucket.go provides a histogram bucket type that accumulates
// elements until a given capacity and enacts a given eviction policy upon
// such a condition.

package metrics

import (
	"bytes"
	"container/heap"
	"fmt"
	"github.com/matttproud/golang_instrumentation/utility"
	"math"
	"sort"
	"sync"
	"time"
)

type AccumulatingBucket struct {
	observations   int
	elements       utility.PriorityQueue
	maximumSize    int
	mutex          sync.RWMutex
	evictionPolicy EvictionPolicy
}

func AccumulatingBucketBuilder(evictionPolicy EvictionPolicy, maximumSize int) BucketBuilder {
	return func() Bucket {
		return &AccumulatingBucket{
			maximumSize:    maximumSize,
			evictionPolicy: evictionPolicy,
			elements:       make(utility.PriorityQueue, 0, maximumSize),
		}
	}
}

// Add a value to the bucket.  Depending on whether the bucket is full, it may
// trigger an eviction of older items.
func (b *AccumulatingBucket) Add(value float64) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.observations++
	size := len(b.elements)

	v := utility.Item{
		Value:    value,
		Priority: -1 * time.Now().UnixNano(),
	}

	if size == b.maximumSize {
		b.evictionPolicy(&b.elements)
	}

	heap.Push(&b.elements, &v)
}

func (b *AccumulatingBucket) Humanize() string {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	buffer := new(bytes.Buffer)

	fmt.Fprintf(buffer, "[AccumulatingBucket with %d elements and %d capacity] { ", len(b.elements), b.maximumSize)

	for i := 0; i < len(b.elements); i++ {
		fmt.Fprintf(buffer, "%f, ", b.elements[i].Value)
	}

	fmt.Fprintf(buffer, "}")

	return string(buffer.Bytes())
}

func (b *AccumulatingBucket) ValueForIndex(index int) float64 {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	elementCount := len(b.elements)

	if elementCount == 0 {
		return math.NaN()
	}

	rawData := make([]float64, elementCount)

	for i, element := range b.elements {
		rawData[i] = element.Value.(float64)
	}

	sort.Float64s(rawData)

	// N.B.(mtp): Interfacing components should not need to comprehend what
	//            evictions strategy is used; therefore, we adjust this silently.
	if index >= elementCount {
		return rawData[elementCount-1]
	}

	return rawData[index]
}

func (b *AccumulatingBucket) Observations() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.observations
}
