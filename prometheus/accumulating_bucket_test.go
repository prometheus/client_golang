// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	. "github.com/matttproud/gocheck"
	"time"
)

func (s *S) TestAccumulatingBucketBuilderWithEvictOldest(c *C) {
	var evictOldestThree EvictionPolicy = EvictOldest(3)

	c.Assert(evictOldestThree, Not(IsNil))

	bb := AccumulatingBucketBuilder(evictOldestThree, 5)

	c.Assert(bb, Not(IsNil))

	var b Bucket = bb()

	c.Assert(b, Not(IsNil))
	c.Check(b.String(), Equals, "[AccumulatingBucket with 0 elements and 5 capacity] { }")

	b.Add(1)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 1 elements and 5 capacity] { 1.000000, }")

	b.Add(2)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 2 elements and 5 capacity] { 1.000000, 2.000000, }")

	b.Add(3)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 3 elements and 5 capacity] { 1.000000, 2.000000, 3.000000, }")

	b.Add(4)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 4 elements and 5 capacity] { 1.000000, 2.000000, 3.000000, 4.000000, }")

	b.Add(5)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 5 elements and 5 capacity] { 1.000000, 2.000000, 3.000000, 4.000000, 5.000000, }")

	b.Add(6)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 3 elements and 5 capacity] { 4.000000, 5.000000, 6.000000, }")

	var bucket Bucket = b

	c.Assert(bucket, Not(IsNil))
}

func (s *S) TestAccumulatingBucketBuilderWithEvictAndReplaceWithAverage(c *C) {
	var evictAndReplaceWithAverage EvictionPolicy = EvictAndReplaceWith(3, AverageReducer)

	c.Assert(evictAndReplaceWithAverage, Not(IsNil))

	bb := AccumulatingBucketBuilder(evictAndReplaceWithAverage, 5)

	c.Assert(bb, Not(IsNil))

	var b Bucket = bb()

	c.Assert(b, Not(IsNil))

	c.Check(b.String(), Equals, "[AccumulatingBucket with 0 elements and 5 capacity] { }")

	b.Add(1)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 1 elements and 5 capacity] { 1.000000, }")

	b.Add(2)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 2 elements and 5 capacity] { 1.000000, 2.000000, }")

	b.Add(3)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 3 elements and 5 capacity] { 1.000000, 2.000000, 3.000000, }")

	b.Add(4)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 4 elements and 5 capacity] { 1.000000, 2.000000, 3.000000, 4.000000, }")

	b.Add(5)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 5 elements and 5 capacity] { 1.000000, 2.000000, 3.000000, 4.000000, 5.000000, }")

	b.Add(6)
	c.Check(b.String(), Equals, "[AccumulatingBucket with 4 elements and 5 capacity] { 4.000000, 5.000000, 2.000000, 6.000000, }")
}

func (s *S) TestAccumulatingBucket(c *C) {
	var b AccumulatingBucket = AccumulatingBucket{
		elements:    make(priorityQueue, 0, 10),
		maximumSize: 5,
	}

	c.Check(b.elements, HasLen, 0)
	c.Check(b.observations, Equals, 0)
	c.Check(b.Observations(), Equals, 0)

	b.Add(5.0)

	c.Check(b.elements, HasLen, 1)
	c.Check(b.observations, Equals, 1)
	c.Check(b.Observations(), Equals, 1)

	b.Add(6.0)
	b.Add(7.0)
	b.Add(8.0)
	b.Add(9.0)

	c.Check(b.elements, HasLen, 5)
	c.Check(b.observations, Equals, 5)
	c.Check(b.Observations(), Equals, 5)
}

func (s *S) TestAccumulatingBucketValueForIndex(c *C) {
	var b AccumulatingBucket = AccumulatingBucket{
		elements:       make(priorityQueue, 0, 100),
		maximumSize:    100,
		evictionPolicy: EvictOldest(50),
	}

	for i := 0; i <= 100; i++ {
		c.Assert(b.ValueForIndex(i), IsNaN)
	}

	// The bucket has only observed one item and contains now one item.
	b.Add(1.0)

	c.Check(b.ValueForIndex(0), Equals, 1.0)
	// Let's sanity check what occurs if presumably an eviction happened and
	// we requested an index larger than what is contained.
	c.Check(b.ValueForIndex(1), Equals, 1.0)

	for i := 2.0; i <= 100; i += 1 {
		b.Add(i)

		// TODO(mtp): This is a sin.  Provide a mechanism for deterministic testing.
		time.Sleep(1 * time.Millisecond)
	}

	c.Check(b.ValueForIndex(0), Equals, 1.0)
	c.Check(b.ValueForIndex(50), Equals, 50.0)
	c.Check(b.ValueForIndex(100), Equals, 100.0)

	for i := 101.0; i <= 150; i += 1 {
		b.Add(i)
		// TODO(mtp): This is a sin.  Provide a mechanism for deterministic testing.
		time.Sleep(1 * time.Millisecond)
	}

	// The bucket's capacity has been exceeded by inputs at this point;
	// consequently, we search for a given element by percentage offset
	// therein.
	c.Check(b.ValueForIndex(0), Equals, 51.0)
	c.Check(b.ValueForIndex(50), Equals, 84.0)
	c.Check(b.ValueForIndex(99), Equals, 116.0)
	c.Check(b.ValueForIndex(100), Equals, 117.0)
}
