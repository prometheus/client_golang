/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package metrics

import (
	. "launchpad.net/gocheck"
)

func (s *S) TestGaugeCreate(c *C) {
	m := GaugeMetric{value: 1.0}

	c.Assert(m, Not(IsNil))
	c.Check(m.Get(), Equals, 1.0)
}

func (s *S) TestGaugeString(c *C) {
	m := GaugeMetric{value: 2.0}
	c.Check(m.String(), Equals, "[GaugeMetric; value=2.000000]")
}

func (s *S) TestGaugeSet(c *C) {
	m := GaugeMetric{value: -1.0}

	m.Set(-99.0)

	c.Check(m.Get(), Equals, -99.0)
}

func (s *S) TestGaugeMetricMarshallable(c *C) {
	m := GaugeMetric{value: 1.0}

	returned := m.Marshallable()

	c.Assert(returned, Not(IsNil))

	c.Check(returned, HasLen, 2)
	c.Check(returned["value"], Equals, 1.0)
	c.Check(returned["type"], Equals, "gauge")
}

func (s *S) TestGaugeAsMetric(c *C) {
	var metric Metric = &GaugeMetric{value: 1.0}

	c.Assert(metric, Not(IsNil))
}
