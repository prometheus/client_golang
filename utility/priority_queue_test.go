// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utility

import (
	"container/heap"
	. "github.com/matttproud/gocheck"
)

func (s *S) TestPriorityQueueSort(c *C) {
	q := make(PriorityQueue, 0, 6)

	c.Check(len(q), Equals, 0)

	heap.Push(&q, &Item{Value: "newest", Priority: -100})
	heap.Push(&q, &Item{Value: "older", Priority: 90})
	heap.Push(&q, &Item{Value: "oldest", Priority: 100})
	heap.Push(&q, &Item{Value: "newer", Priority: -90})
	heap.Push(&q, &Item{Value: "new", Priority: -80})
	heap.Push(&q, &Item{Value: "old", Priority: 80})

	c.Check(len(q), Equals, 6)

	c.Check(heap.Pop(&q), ValueEquals, "oldest")
	c.Check(heap.Pop(&q), ValueEquals, "older")
	c.Check(heap.Pop(&q), ValueEquals, "old")
	c.Check(heap.Pop(&q), ValueEquals, "new")
	c.Check(heap.Pop(&q), ValueEquals, "newer")
	c.Check(heap.Pop(&q), ValueEquals, "newest")
}
