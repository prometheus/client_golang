// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metrics

import (
	. "github.com/matttproud/gocheck"
	"github.com/prometheus/client_golang/maths"
)

func (s *S) TestTallyingPercentileEstimatorMinimum(c *C) {
	c.Assert(Minimum(-2, -1, 0, 0), maths.IsNaN)
	c.Check(Minimum(-2, -1, 0, 1), Equals, -2.0)
}

func (s *S) TestTallyingPercentileEstimatorMaximum(c *C) {
	c.Assert(Maximum(-2, -1, 0, 0), maths.IsNaN)
	c.Check(Maximum(-2, -1, 0, 1), Equals, -1.0)
}

func (s *S) TestTallyingPercentilesEstimatorAverage(c *C) {
	c.Assert(Average(-2, -1, 0, 0), maths.IsNaN)
	c.Check(Average(-2, -2, 0, 1), Equals, -2.0)
	c.Check(Average(-1, -1, 0, 1), Equals, -1.0)
	c.Check(Average(1, 1, 0, 2), Equals, 1.0)
	c.Check(Average(2, 1, 0, 2), Equals, 1.5)
}

func (s *S) TestTallyingPercentilesEstimatorUniform(c *C) {
	c.Assert(Uniform(-5, 5, 0, 0), maths.IsNaN)

	c.Check(Uniform(-5, 5, 0, 2), Equals, -5.0)
	c.Check(Uniform(-5, 5, 1, 2), Equals, 0.0)
	c.Check(Uniform(-5, 5, 2, 2), Equals, 5.0)
}

func (s *S) TestTallyingBucketBuilder(c *C) {
	var bucket Bucket = tallyingBucketBuilder()

	c.Assert(bucket, Not(IsNil))
}

func (s *S) TestTallyingBucketString(c *C) {
	bucket := TallyingBucket{
		observations:     3,
		smallestObserved: 2.0,
		largestObserved:  5.5,
	}

	c.Check(bucket.String(), Equals, "[TallyingBucket (2.000000, 5.500000); 3 items]")
}

func (s *S) TestTallyingBucketAdd(c *C) {
	b := DefaultTallyingBucket()

	b.Add(1)

	c.Check(b.observations, Equals, 1)
	c.Check(b.Observations(), Equals, 1)
	c.Check(b.smallestObserved, Equals, 1.0)
	c.Check(b.largestObserved, Equals, 1.0)

	b.Add(2)

	c.Check(b.observations, Equals, 2)
	c.Check(b.Observations(), Equals, 2)
	c.Check(b.smallestObserved, Equals, 1.0)
	c.Check(b.largestObserved, Equals, 2.0)
}
