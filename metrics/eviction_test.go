// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// eviction_test.go provides a test complement for the eviction.go module.

package metrics

import (
	"container/heap"
	"github.com/matttproud/golang_instrumentation/maths"
	"github.com/matttproud/golang_instrumentation/utility"
	. "launchpad.net/gocheck"
)

func (s *S) TestEvictOldest(c *C) {
	q := make(utility.PriorityQueue, 0, 10)
	heap.Init(&q)
	var e EvictionPolicy = EvictOldest(5)

	for i := 0; i < 10; i++ {
		var item utility.Item = utility.Item{
			Value:    float64(i),
			Priority: int64(i),
		}

		heap.Push(&q, &item)
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 5)

	c.Check(heap.Pop(&q), utility.ValueEquals, 4.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 3.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 2.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 1.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 0.0)
}

// TODO(mtp): Extract reduction mechanisms into local variables.

func (s *S) TestEvictAndReplaceWithAverage(c *C) {
	q := make(utility.PriorityQueue, 0, 10)
	heap.Init(&q)
	var e EvictionPolicy = EvictAndReplaceWith(5, maths.Average)

	for i := 0; i < 10; i++ {
		var item utility.Item = utility.Item{
			Value:    float64(i),
			Priority: int64(i),
		}

		heap.Push(&q, &item)
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 6)

	c.Check(heap.Pop(&q), utility.ValueEquals, 4.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 3.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 2.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 1.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 0.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 7.0)
}

func (s *S) TestEvictAndReplaceWithMedian(c *C) {
	q := make(utility.PriorityQueue, 0, 10)
	heap.Init(&q)
	var e EvictionPolicy = EvictAndReplaceWith(5, maths.Median)

	for i := 0; i < 10; i++ {
		var item utility.Item = utility.Item{
			Value:    float64(i),
			Priority: int64(i),
		}

		heap.Push(&q, &item)
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 6)

	c.Check(heap.Pop(&q), utility.ValueEquals, 4.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 3.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 2.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 1.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 0.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 7.0)
}

func (s *S) TestEvictAndReplaceWithFirstMode(c *C) {
	q := make(utility.PriorityQueue, 0, 10)
	heap.Init(&q)
	e := EvictAndReplaceWith(5, maths.FirstMode)

	for i := 0; i < 10; i++ {
		heap.Push(&q, &utility.Item{
			Value:    float64(i),
			Priority: int64(i),
		})
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 6)

	c.Check(heap.Pop(&q), utility.ValueEquals, 4.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 3.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 2.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 1.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 0.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 9.0)
}

func (s *S) TestEvictAndReplaceWithMinimum(c *C) {
	q := make(utility.PriorityQueue, 0, 10)
	heap.Init(&q)
	var e EvictionPolicy = EvictAndReplaceWith(5, maths.Minimum)

	for i := 0; i < 10; i++ {
		var item utility.Item = utility.Item{
			Value:    float64(i),
			Priority: int64(i),
		}

		heap.Push(&q, &item)
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 6)

	c.Check(heap.Pop(&q), utility.ValueEquals, 4.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 3.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 2.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 1.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 0.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 5.0)
}

func (s *S) TestEvictAndReplaceWithMaximum(c *C) {
	q := make(utility.PriorityQueue, 0, 10)
	heap.Init(&q)
	var e EvictionPolicy = EvictAndReplaceWith(5, maths.Maximum)

	for i := 0; i < 10; i++ {
		var item utility.Item = utility.Item{
			Value:    float64(i),
			Priority: int64(i),
		}

		heap.Push(&q, &item)
	}

	c.Check(q, HasLen, 10)

	e(&q)

	c.Check(q, HasLen, 6)

	c.Check(heap.Pop(&q), utility.ValueEquals, 4.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 3.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 2.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 1.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 0.0)
	c.Check(heap.Pop(&q), utility.ValueEquals, 9.0)
}
