// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"sort"
	"sync"

	dto "github.com/prometheus/client_model/go"

	"code.google.com/p/goprotobuf/proto"
)

type Counter interface {
	Metric

	// XXX
	Inc()
	Dec()
	Add(float64)
	Sub(float64)
	Set(float64)
}

type CounterDesc struct {
	Desc
}

func (d CounterDesc) build() Counter {
	return &counter{
		desc: d,
	}
}

type counter struct {
	mtx  sync.RWMutex
	val  float64
	desc CounterDesc
}

func (c *counter) Desc() Desc {
	return c.desc.Desc
}

func (c *counter) Set(v float64) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.val = v
}

func (c *counter) Inc() {
	c.Add(1)
}

func (c *counter) Dec() {
	c.Add(-1)
}

func (c *counter) Add(v float64) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.val += v
}

func (c *counter) Sub(v float64) {
	c.Add(v * -1)
}

func (c *counter) Write(out *dto.MetricFamily) {
	c.mtx.RLock()
	gg := &dto.Counter{
		Value: proto.Float64(c.val),
	}
	c.mtx.RUnlock()

	out.Type = dto.MetricType_COUNTER.Enum()

	out.Metric = append(out.Metric, &dto.Metric{
		Counter: gg,
	})
}

func NewCounter(desc CounterDesc) Counter {
	return desc.build()
}

type CounterVec interface {
	Metric

	// XXX
	Inc(...string)
	Dec(...string)
	Add(float64, ...string)
	Sub(float64, ...string)
	Set(float64, ...string)

	Del(...string) bool
}

type CounterVecDesc struct {
	Desc

	Labels []string // XXX
}

func (d *CounterVecDesc) build() CounterVec {
	if len(d.Labels) == 0 {
		panic(errZeroCardinalityForVec)
	}
	ls := map[string]bool{}
	for _, l := range d.Labels {
		if l == "" {
			panic(errEmptyLabelDesc)
		}
		ls[l] = true
	}
	if len(ls) != len(d.Labels) {
		panic(errDuplLabelDesc)
	}

	return &counterVec{
		desc:     *d,
		children: make(map[uint64]*counterVecElem),
	}
}

type counterVecElem struct {
	desc CounterVecDesc
	mtx  sync.RWMutex
	val  float64
	dims []string
}

func (c *counterVecElem) Set(v float64) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.val = v
}

func (c *counterVecElem) Inc() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.val++
}

func (c *counterVecElem) Dec() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.val--
}

func (c *counterVecElem) Add(v float64) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.val += v
}

func (c *counterVecElem) Sub(v float64) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.val -= v
}

func (c *counterVecElem) Get() float64 {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	return c.val
}

func (c *counterVecElem) Write(o *dto.Metric) {
	o.Counter = &dto.Counter{
		Value: proto.Float64(c.Get()),
	}
	dims := make([]*dto.LabelPair, 0, len(c.desc.Labels))
	for i, n := range c.desc.Labels {
		dims = append(dims, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(c.dims[i]),
		})
	}
	sort.Sort(lpSorter(dims))
	o.Label = dims
}

type counterVec struct {
	CounterVec

	desc     CounterVecDesc
	mtx      sync.RWMutex
	children map[uint64]*counterVecElem
}

func (c *counterVec) Desc() Desc {
	return c.desc.Desc
}

func (c *counterVec) Write(out *dto.MetricFamily) {
	out.Type = dto.MetricType_COUNTER.Enum()

	c.mtx.RLock()
	elems := map[uint64]*counterVecElem{}
	hashes := make([]uint64, 0, len(elems))
	for h, e := range c.children {
		elems[h] = e
		hashes = append(hashes, h)
	}
	c.mtx.RUnlock()

	sort.Sort(hashSorter(hashes))

	cs := make([]*dto.Metric, 0, len(hashes))
	for _, h := range hashes {
		c := new(dto.Metric)
		elems[h].Write(c)
		cs = append(cs, c)
	}

	out.Metric = cs
}

func (c *counterVec) Set(v float64, dims ...string) {
	if len(dims) != len(c.desc.Labels) || len(dims) == 0 {
		panic(errInconsistentCardinality)
	}

	h := hashLabelValues(dims...)

	c.mtx.RLock()
	if vec, ok := c.children[h]; ok {
		c.mtx.RUnlock()
		vec.Set(v)
		return
	}
	c.mtx.RUnlock()

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if vec, ok := c.children[h]; ok {
		vec.Set(v)
		return
	}
	c.children[h] = &counterVecElem{
		val:  v,
		dims: dims,
		desc: c.desc,
	}
}

func (c *counterVec) Inc(dims ...string) {
	if len(dims) != len(c.desc.Labels) || len(dims) == 0 {
		panic(errInconsistentCardinality)
	}

	h := hashLabelValues(dims...)

	c.mtx.RLock()
	if vec, ok := c.children[h]; ok {
		c.mtx.RUnlock()
		vec.Inc()
		return
	}
	c.mtx.RUnlock()

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if vec, ok := c.children[h]; ok {
		vec.Inc()
		return
	}
	c.children[h] = &counterVecElem{
		val:  1,
		dims: dims,
		desc: c.desc,
	}
	c.children[h].Inc()
}

func (c *counterVec) Dec(dims ...string) {
	if len(dims) != len(c.desc.Labels) || len(dims) == 0 {
		panic(errInconsistentCardinality)
	}

	h := hashLabelValues(dims...)

	c.mtx.RLock()
	if vec, ok := c.children[h]; ok {
		c.mtx.RUnlock()
		vec.Dec()
		return
	}
	c.mtx.RUnlock()

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if vec, ok := c.children[h]; ok {
		vec.Dec()
		return
	}
	c.children[h] = &counterVecElem{
		val:  -1,
		dims: dims,
		desc: c.desc,
	}
	c.children[h].Dec()
}

func (c *counterVec) Add(v float64, dims ...string) {
	if len(dims) != len(c.desc.Labels) || len(dims) == 0 {
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
	c.children[h] = &counterVecElem{
		val:  v,
		dims: dims,
		desc: c.desc,
	}
	c.children[h].Add(v)
}

func (c *counterVec) Sub(v float64, dims ...string) {
	if len(dims) != len(c.desc.Labels) || len(dims) == 0 {
		panic(errInconsistentCardinality)
	}

	h := hashLabelValues(dims...)

	c.mtx.RLock()
	if vec, ok := c.children[h]; ok {
		c.mtx.RUnlock()
		vec.Sub(v)
		return
	}
	c.mtx.RUnlock()

	c.mtx.Lock()
	defer c.mtx.Unlock()
	if vec, ok := c.children[h]; ok {
		vec.Sub(v)
		return
	}
	c.children[h] = &counterVecElem{
		val:  -v,
		dims: dims,
		desc: c.desc,
	}
	c.children[h].Sub(v)
}

func NewCounterVec(desc CounterVecDesc) CounterVec {
	return desc.build()
}
