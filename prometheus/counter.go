// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import dto "github.com/prometheus/client_model/go"

type Counter interface {
	Metric
	MetricsCollector

	Inc(...string)
	Dec(...string)
	Add(float64, ...string)
	Sub(float64, ...string)
	Set(float64, ...string)
	// Del deletes a given label set from this Counter, indicating whether the
	// label set was deleted.
	Del(...string) bool
}

// NewCounter emits a new Counter from the provided descriptor.
// The Type field is ignored and forcefully set to MetricType_COUNTER.
func NewCounter(desc *Desc) Counter {
	desc.Type = dto.MetricType_COUNTER
	if len(desc.VariableLabels) == 0 {
		result := &counter{valueMetric: valueMetric{desc: desc}}
		result.Self = result
		return result
	}
	result := &counterVec{
		valueMetricVec: valueMetricVec{
			desc:     desc,
			children: map[uint64]*valueMetricVecElem{},
		},
	}
	result.Self = result
	return result
}

type counter struct {
	valueMetric
}

func (c *counter) Inc(dims ...string) {
	c.Add(1, dims...)
}

func (c *counter) Dec(dims ...string) {
	c.Add(-1, dims...)
}

func (c *counter) Add(v float64, dims ...string) {
	if len(dims) != 0 {
		panic(errInconsistentCardinality)
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.val += v
}

func (c *counter) Sub(v float64, dims ...string) {
	c.Add(v*-1, dims...)
}

type counterVec struct {
	valueMetricVec
}

func (c *counterVec) Inc(dims ...string) {
	c.Add(1, dims...)
}

func (c *counterVec) Dec(dims ...string) {
	c.Add(-1, dims...)
}

func (c *counterVec) Add(v float64, dims ...string) {
	if len(dims) != len(c.desc.VariableLabels) {
		panic(errInconsistentCardinality)
	}

	h := hashLabelValues(dims...)

	c.mtx.RLock()
	if vec, ok := c.children[h]; ok {
		c.mtx.RUnlock()
		vec.Add(v)
		return
	}
	c.mtx.RUnlock()

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if vec, ok := c.children[h]; ok {
		vec.Add(v)
		return
	}
	c.children[h] = &valueMetricVecElem{
		val:  v,
		dims: dims,
		desc: c.desc,
	}
}

func (c *counterVec) Sub(v float64, dims ...string) {
	c.Add(v*-1, dims...)
}
