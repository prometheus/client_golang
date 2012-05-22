// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// histogram_test.go provides a test complement for the histogram.go module.

package metrics

import (
	"github.com/matttproud/golang_instrumentation/maths"
	. "launchpad.net/gocheck"
)

func (s *S) TestEquallySizedBucketsFor(c *C) {
	h := EquallySizedBucketsFor(0, 10, 5)

	c.Assert(h, Not(IsNil))
	c.Check(h, HasLen, 5)
	c.Check(h[0], Equals, 0.0)
	c.Check(h[1], Equals, 2.0)
	c.Check(h[2], Equals, 4.0)
	c.Check(h[3], Equals, 6.0)
	c.Check(h[4], Equals, 8.0)
}

func (s *S) TestLogarithmicSizedBucketsFor(c *C) {
	h := LogarithmicSizedBucketsFor(0, 2048)

	c.Assert(h, Not(IsNil))
	c.Check(h, HasLen, 11)
	c.Check(h[0], Equals, 0.0)
	c.Check(h[1], Equals, 2.0)
	c.Check(h[2], Equals, 4.0)
	c.Check(h[3], Equals, 8.0)
	c.Check(h[4], Equals, 16.0)
	c.Check(h[5], Equals, 32.0)
	c.Check(h[6], Equals, 64.0)
	c.Check(h[7], Equals, 128.0)
	c.Check(h[8], Equals, 256.0)
	c.Check(h[9], Equals, 512.0)
	c.Check(h[10], Equals, 1024.0)
}

func (s *S) TestCreateHistogram(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 10, 5),
		BucketMaker: TallyingBucketBuilder,
	}

	h := CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	c.Check(h.Humanize(), Equals, "[Histogram { [0.000000, inf) = [TallyingBucket (Empty)], [2.000000, inf) = [TallyingBucket (Empty)], [4.000000, inf) = [TallyingBucket (Empty)], [6.000000, inf) = [TallyingBucket (Empty)], [8.000000, inf) = [TallyingBucket (Empty)], }]")

	h.Add(1)

	c.Check(h.Humanize(), Equals, "[Histogram { [0.000000, inf) = [TallyingBucket (1.000000, 1.000000); 1 items], [2.000000, inf) = [TallyingBucket (Empty)], [4.000000, inf) = [TallyingBucket (Empty)], [6.000000, inf) = [TallyingBucket (Empty)], [8.000000, inf) = [TallyingBucket (Empty)], }]")
}

func (s *S) TestBucketForPercentile(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 100, 100),
		BucketMaker: TallyingBucketBuilder,
	}

	h := CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	var bucket *Bucket = nil
	var subindex int = 0

	for i := 0.0; i < 1.0; i += 0.01 {
		bucket, subindex := h.bucketForPercentile(i)

		c.Assert(*bucket, Not(IsNil))
		c.Check(subindex, Equals, 0)
	}

	h.Add(1.0)

	bucket, subindex = h.bucketForPercentile(0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	for i := 2.0; i <= 100.0; i++ {
		h.Add(i)
	}

	bucket, subindex = h.bucketForPercentile(0.05)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	for i := 0; i < 50; i++ {
		h.Add(50)
		h.Add(51)
	}

	bucket, subindex = h.bucketForPercentile(0.50)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 50)
	c.Check((*bucket).Observations(), Equals, 51)

	bucket, subindex = h.bucketForPercentile(0.51)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 51)
}

func (s *S) TestBucketForPercentileSingleton(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 3, 3),
		BucketMaker: TallyingBucketBuilder,
	}

	var h *Histogram = CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	var bucket *Bucket = nil
	var subindex int = 0

	for i := 0.0; i < 1.0; i += 0.01 {
		bucket, subindex := h.bucketForPercentile(i)

		c.Assert(*bucket, Not(IsNil))
		c.Check(subindex, Equals, 0)
	}

	h.Add(0.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	h = CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	for i := 0.0; i < 1.0; i += 0.01 {
		bucket, subindex := h.bucketForPercentile(i)

		c.Assert(*bucket, Not(IsNil))
		c.Check(subindex, Equals, 0)
	}

	h.Add(1.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	h = CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	for i := 0.0; i < 1.0; i += 0.01 {
		bucket, subindex := h.bucketForPercentile(i)

		c.Assert(*bucket, Not(IsNil))
		c.Check(subindex, Equals, 0)
	}

	h.Add(2.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)
}

func (s *S) TestBucketForPercentileDoubleInSingleBucket(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 3, 3),
		BucketMaker: TallyingBucketBuilder,
	}

	var h *Histogram = CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	var bucket *Bucket = nil
	var subindex int = 0

	for i := 0.0; i < 1.0; i += 0.01 {
		bucket, subindex := h.bucketForPercentile(i)

		c.Assert(*bucket, Not(IsNil))
		c.Check(subindex, Equals, 0)
	}

	h.Add(0.0)
	h.Add(0.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	h = CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	for i := 0.0; i < 1.0; i += 0.01 {
		bucket, subindex := h.bucketForPercentile(i)

		c.Assert(*bucket, Not(IsNil))
		c.Check(subindex, Equals, 0)
	}

	h.Add(1.0)
	h.Add(1.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	h = CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	for i := 0.0; i < 1.0; i += 0.01 {
		bucket, subindex := h.bucketForPercentile(i)

		c.Assert(*bucket, Not(IsNil))
		c.Check(subindex, Equals, 0)
	}

	h.Add(2.0)
	h.Add(2.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)
}

func (s *S) TestBucketForPercentileTripleInSingleBucket(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 3, 3),
		BucketMaker: TallyingBucketBuilder,
	}

	var h *Histogram = CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	var bucket *Bucket = nil
	var subindex int = 0

	for i := 0.0; i < 1.0; i += 0.01 {
		bucket, subindex := h.bucketForPercentile(i)

		c.Assert(*bucket, Not(IsNil))
		c.Check(subindex, Equals, 0)
	}

	h.Add(0.0)
	h.Add(0.0)
	h.Add(0.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 2)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(0.67)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(2.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(1.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 3)

	h = CreateHistogram(hs)

	h.Add(1.0)
	h.Add(1.0)
	h.Add(1.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 2)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(0.67)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(2.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(1.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 3)

	h = CreateHistogram(hs)

	h.Add(2.0)
	h.Add(2.0)
	h.Add(2.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 2)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(0.67)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(2.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(1.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 3)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 3)
}

func (s *S) TestBucketForPercentileTwoEqualAdjacencies(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 3, 3),
		BucketMaker: TallyingBucketBuilder,
	}

	var h *Histogram = CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	var bucket *Bucket = nil
	var subindex int = 0

	for i := 0.0; i < 1.0; i += 0.01 {
		bucket, subindex := h.bucketForPercentile(i)

		c.Assert(*bucket, Not(IsNil))
		c.Check(subindex, Equals, 0)
	}

	h.Add(0.0)
	h.Add(1.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.67)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(2.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(1.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	h = CreateHistogram(hs)

	h.Add(1.0)
	h.Add(2.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.67)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(2.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(1.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)
}

func (s *S) TestBucketForPercentileTwoAdjacenciesUnequal(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 3, 3),
		BucketMaker: TallyingBucketBuilder,
	}

	var h *Histogram = CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	var bucket *Bucket = nil
	var subindex int = 0

	for i := 0.0; i < 1.0; i += 0.01 {
		bucket, subindex := h.bucketForPercentile(i)

		c.Assert(*bucket, Not(IsNil))
		c.Check(subindex, Equals, 0)
	}

	h.Add(0.0)
	h.Add(0.0)
	h.Add(1.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.67)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(2.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(1.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	h = CreateHistogram(hs)

	h.Add(0.0)
	h.Add(1.0)
	h.Add(1.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.67)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(2.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(1.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	h = CreateHistogram(hs)

	h.Add(1.0)
	h.Add(1.0)
	h.Add(2.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.67)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(2.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(1.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	h = CreateHistogram(hs)

	h.Add(1.0)
	h.Add(2.0)
	h.Add(2.0)

	bucket, subindex = h.bucketForPercentile(1.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 1)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.67)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(2.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(0.5)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 2)

	bucket, subindex = h.bucketForPercentile(1.0 / 3.0)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(0.01)

	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)
}

func (s *S) TestBucketForPercentileWithBinomialApproximation(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 5, 6),
		BucketMaker: TallyingBucketBuilder,
	}

	c.Assert(hs, Not(IsNil))

	h := CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	n := 5
	p := 0.5

	for k := 0; k < 6; k++ {
		limit := 1000000.0 * maths.BinomialPDF(k, n, p)
		for j := 0.0; j < limit; j++ {
			h.Add(float64(k))
		}
	}

	var bucket *Bucket = nil
	var subindex int = 0

	bucket, subindex = h.bucketForPercentile(0.0)
	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 31250)

	bucket, subindex = h.bucketForPercentile(0.03125)
	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 31249)
	c.Check((*bucket).Observations(), Equals, 31250)

	bucket, subindex = h.bucketForPercentile(0.1875)
	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 156249)
	c.Check((*bucket).Observations(), Equals, 156250)

	bucket, subindex = h.bucketForPercentile(0.50)
	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 312499)
	c.Check((*bucket).Observations(), Equals, 312500)

	bucket, subindex = h.bucketForPercentile(0.8125)
	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 312499)
	c.Check((*bucket).Observations(), Equals, 312500)

	bucket, subindex = h.bucketForPercentile(0.96875)
	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 156249)
	c.Check((*bucket).Observations(), Equals, 156250)

	bucket, subindex = h.bucketForPercentile(1.0)
	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 31249)
	c.Check((*bucket).Observations(), Equals, 31250)
}

func (s *S) TestBucketForPercentileWithUniform(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 100, 100),
		BucketMaker: TallyingBucketBuilder,
	}

	c.Assert(hs, Not(IsNil))

	h := CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	for i := 0.0; i <= 99.0; i++ {
		h.Add(i)
	}

	for i := 0; i <= 99; i++ {
		c.Check(h.bucketStarts[i], Equals, float64(i))
	}

	for i := 1; i <= 100; i++ {
		c.Check(h.buckets[i-1].Observations(), Equals, 1)
	}

	var bucket *Bucket = nil
	var subindex int = 0

	bucket, subindex = h.bucketForPercentile(0.01)
	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)

	bucket, subindex = h.bucketForPercentile(1.0)
	c.Assert(*bucket, Not(IsNil))
	c.Check(subindex, Equals, 0)
	c.Check((*bucket).Observations(), Equals, 1)
}

func (s *S) TestHistogramPercentileUniform(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 100, 100),
		BucketMaker: TallyingBucketBuilder,
	}

	h := CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	for i := 0.0; i <= 99.0; i++ {
		h.Add(i)
	}

	c.Check(h.Percentile(0.01), Equals, 0.0)
	c.Check(h.Percentile(0.49), Equals, 48.0)
	c.Check(h.Percentile(0.50), Equals, 49.0)
	c.Check(h.Percentile(0.51), Equals, 50.0)
	c.Check(h.Percentile(1.0), Equals, 99.0)
}

func (s *S) TestHistogramPercentileBinomialApproximation(c *C) {
	hs := &HistogramSpecification{
		Starts:      EquallySizedBucketsFor(0, 5, 6),
		BucketMaker: TallyingBucketBuilder,
	}

	h := CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	n := 5
	p := 0.5

	for k := 0; k < 6; k++ {
		limit := 1000000.0 * maths.BinomialPDF(k, n, p)
		for j := 0.0; j < limit; j++ {
			h.Add(float64(k))
		}
	}

	c.Check(h.Percentile(0.0), Equals, 0.0)
	c.Check(h.Percentile(0.03125), Equals, 0.0)
	c.Check(h.Percentile(0.1875), Equals, 1.0)
	c.Check(h.Percentile(0.5), Equals, 2.0)
	c.Check(h.Percentile(0.8125), Equals, 3.0)
	c.Check(h.Percentile(0.96875), Equals, 4.0)
	c.Check(h.Percentile(1.0), Equals, 5.0)
}

func (s *S) TestHistogramMarshallable(c *C) {
	hs := &HistogramSpecification{
		Starts:                EquallySizedBucketsFor(0, 5, 6),
		BucketMaker:           TallyingBucketBuilder,
		ReportablePercentiles: []float64{0.03125, 0.1875, 0.5, 0.8125, 0.96875, 1.0},
	}

	h := CreateHistogram(hs)

	c.Assert(h, Not(IsNil))

	n := 5
	p := 0.5

	for k := 0; k < 6; k++ {
		limit := 1000000.0 * maths.BinomialPDF(k, n, p)
		for j := 0.0; j < limit; j++ {
			h.Add(float64(k))
		}
	}

	m := h.Marshallable()

	c.Assert(m, Not(IsNil))
	c.Check(m, HasLen, 2)
	c.Check(m["type"], Equals, "histogram")

	var v map[string]interface{} = m["value"].(map[string]interface{})

	c.Assert(v, Not(IsNil))

	c.Check(v, HasLen, 6)
	c.Check(v["0.031250"], Equals, "0.000000")
	c.Check(v["0.187500"], Equals, "1.000000")
	c.Check(v["0.500000"], Equals, "2.000000")
	c.Check(v["0.812500"], Equals, "3.000000")
	c.Check(v["0.968750"], Equals, "4.000000")
	c.Check(v["1.000000"], Equals, "5.000000")
}

func (s *S) TestHistogramAsMetric(c *C) {
	hs := &HistogramSpecification{
		Starts:                EquallySizedBucketsFor(0, 5, 6),
		BucketMaker:           TallyingBucketBuilder,
		ReportablePercentiles: []float64{0.0, 0.03125, 0.1875, 0.5, 0.8125, 0.96875, 1.0},
	}

	h := CreateHistogram(hs)

	var metric Metric = h

	c.Assert(metric, Not(IsNil))
}
