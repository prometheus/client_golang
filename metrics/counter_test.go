/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package metrics

import (
	. "github.com/matttproud/gocheck"
)

func (s *S) TestCounterCreate(c *C) {
	m := CounterMetric{value: 1.0}

	c.Assert(m, Not(IsNil))
}

func (s *S) TestCounterGet(c *C) {
	m := CounterMetric{value: 42.23}

	c.Check(m.Get(), Equals, 42.23)
}

func (s *S) TestCounterSet(c *C) {
	m := CounterMetric{value: 42.23}
	m.Set(40.4)

	c.Check(m.Get(), Equals, 40.4)
}

func (s *S) TestCounterReset(c *C) {
	m := CounterMetric{value: 42.23}
	m.Reset()

	c.Check(m.Get(), Equals, 0.0)
}

func (s *S) TestCounterIncrementBy(c *C) {
	m := CounterMetric{value: 1.0}

	m.IncrementBy(1.5)

	c.Check(m.Get(), Equals, 2.5)
	c.Check(m.String(), Equals, "[CounterMetric; value=2.500000]")
}

func (s *S) TestCounterIncrement(c *C) {
	m := CounterMetric{value: 1.0}

	m.Increment()

	c.Check(m.Get(), Equals, 2.0)
	c.Check(m.String(), Equals, "[CounterMetric; value=2.000000]")
}

func (s *S) TestCounterDecrementBy(c *C) {
	m := CounterMetric{value: 1.0}

	m.DecrementBy(1.0)

	c.Check(m.Get(), Equals, 0.0)
	c.Check(m.String(), Equals, "[CounterMetric; value=0.000000]")
}

func (s *S) TestCounterDecrement(c *C) {
	m := CounterMetric{value: 1.0}

	m.Decrement()

	c.Check(m.Get(), Equals, 0.0)
	c.Check(m.String(), Equals, "[CounterMetric; value=0.000000]")
}

func (s *S) TestCounterString(c *C) {
	m := CounterMetric{value: 2.0}
	c.Check(m.String(), Equals, "[CounterMetric; value=2.000000]")
}

func (s *S) TestCounterMetricMarshallable(c *C) {
	m := CounterMetric{value: 1.0}

	returned := m.Marshallable()

	c.Assert(returned, Not(IsNil))

	c.Check(returned, HasLen, 2)
	c.Check(returned["value"], Equals, 1.0)
	c.Check(returned["type"], Equals, "counter")
}

func (s *S) TestCounterAsMetric(c *C) {
	var metric Metric = &CounterMetric{value: 1.0}

	c.Assert(metric, Not(IsNil))
}
