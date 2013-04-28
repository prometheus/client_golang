// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"container/heap"
	. "github.com/matttproud/gocheck"
)

func (s *S) TestEvictOldest(c *C) {
	q := make(priorityQueue, 0, 10)
	heap.Init(&q)
	var e EvictionPolicy = EvictOldest(5)

	for i := 0; i < 10; i++ {
		var item item = item{
			Priority: int64(i),
			Value:    float64(i),
		}

		heap.Push(&q, &item)
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 5)

	c.Check(heap.Pop(&q), ValueEquals, 4.0)
	c.Check(heap.Pop(&q), ValueEquals, 3.0)
	c.Check(heap.Pop(&q), ValueEquals, 2.0)
	c.Check(heap.Pop(&q), ValueEquals, 1.0)
	c.Check(heap.Pop(&q), ValueEquals, 0.0)
}

func (s *S) TestEvictAndReplaceWithAverage(c *C) {
	q := make(priorityQueue, 0, 10)
	heap.Init(&q)
	var e EvictionPolicy = EvictAndReplaceWith(5, AverageReducer)

	for i := 0; i < 10; i++ {
		var item item = item{
			Priority: int64(i),
			Value:    float64(i),
		}

		heap.Push(&q, &item)
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 6)

	c.Check(heap.Pop(&q), ValueEquals, 4.0)
	c.Check(heap.Pop(&q), ValueEquals, 3.0)
	c.Check(heap.Pop(&q), ValueEquals, 2.0)
	c.Check(heap.Pop(&q), ValueEquals, 1.0)
	c.Check(heap.Pop(&q), ValueEquals, 0.0)
	c.Check(heap.Pop(&q), ValueEquals, 7.0)
}

func (s *S) TestEvictAndReplaceWithMedian(c *C) {
	q := make(priorityQueue, 0, 10)
	heap.Init(&q)
	var e EvictionPolicy = EvictAndReplaceWith(5, MedianReducer)

	for i := 0; i < 10; i++ {
		var item item = item{
			Priority: int64(i),
			Value:    float64(i),
		}

		heap.Push(&q, &item)
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 6)

	c.Check(heap.Pop(&q), ValueEquals, 4.0)
	c.Check(heap.Pop(&q), ValueEquals, 3.0)
	c.Check(heap.Pop(&q), ValueEquals, 2.0)
	c.Check(heap.Pop(&q), ValueEquals, 1.0)
	c.Check(heap.Pop(&q), ValueEquals, 0.0)
	c.Check(heap.Pop(&q), ValueEquals, 7.0)
}

func (s *S) TestEvictAndReplaceWithFirstMode(c *C) {
	q := make(priorityQueue, 0, 10)
	heap.Init(&q)
	e := EvictAndReplaceWith(5, FirstModeReducer)

	for i := 0; i < 10; i++ {
		heap.Push(&q, &item{
			Priority: int64(i),
			Value:    float64(i),
		})
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 6)

	c.Check(heap.Pop(&q), ValueEquals, 4.0)
	c.Check(heap.Pop(&q), ValueEquals, 3.0)
	c.Check(heap.Pop(&q), ValueEquals, 2.0)
	c.Check(heap.Pop(&q), ValueEquals, 1.0)
	c.Check(heap.Pop(&q), ValueEquals, 0.0)
	c.Check(heap.Pop(&q), ValueEquals, 9.0)
}

func (s *S) TestEvictAndReplaceWithMinimum(c *C) {
	q := make(priorityQueue, 0, 10)
	heap.Init(&q)
	var e EvictionPolicy = EvictAndReplaceWith(5, MinimumReducer)

	for i := 0; i < 10; i++ {
		var item item = item{
			Priority: int64(i),
			Value:    float64(i),
		}

		heap.Push(&q, &item)
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 6)

	c.Check(heap.Pop(&q), ValueEquals, 4.0)
	c.Check(heap.Pop(&q), ValueEquals, 3.0)
	c.Check(heap.Pop(&q), ValueEquals, 2.0)
	c.Check(heap.Pop(&q), ValueEquals, 1.0)
	c.Check(heap.Pop(&q), ValueEquals, 0.0)
	c.Check(heap.Pop(&q), ValueEquals, 5.0)
}

func (s *S) TestEvictAndReplaceWithMaximum(c *C) {
	q := make(priorityQueue, 0, 10)
	heap.Init(&q)
	var e EvictionPolicy = EvictAndReplaceWith(5, MaximumReducer)

	for i := 0; i < 10; i++ {
		var item item = item{
			Priority: int64(i),
			Value:    float64(i),
		}

		heap.Push(&q, &item)
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 6)

	c.Check(heap.Pop(&q), ValueEquals, 4.0)
	c.Check(heap.Pop(&q), ValueEquals, 3.0)
	c.Check(heap.Pop(&q), ValueEquals, 2.0)
	c.Check(heap.Pop(&q), ValueEquals, 1.0)
	c.Check(heap.Pop(&q), ValueEquals, 0.0)
	c.Check(heap.Pop(&q), ValueEquals, 9.0)
}
