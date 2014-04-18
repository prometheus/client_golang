// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"sync"

	dto "github.com/prometheus/client_model/go"

	"code.google.com/p/goprotobuf/proto"
)

// XXX: Callback protocol for gauges.

// Gauge proxies a scalar value.
type Gauge interface {
	Metric

	// Set assigns the value of this Gauge to the proxied value.
	Set(float64)
}

// GaugeDesc is the descriptor for a scalar Gauge.
type GaugeDesc struct {
	Desc
}

func (d GaugeDesc) build() Gauge {
	return &gauge{
		desc: d,
	}
}

type gauge struct {
	Gauge

	mtx  sync.RWMutex
	val  float64
	desc GaugeDesc
}

func (g *gauge) Desc() Desc {
	return g.desc.Desc
}

func (g *gauge) Set(v float64) {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	g.val = v
}

func (g *gauge) Write(out *dto.MetricFamily) {
	g.mtx.RLock()
	gDto := &dto.Gauge{
		Value: proto.Float64(g.val),
	}
	g.mtx.RUnlock()

	out.Type = dto.MetricType_GAUGE.Enum()

	out.Metric = append(out.Metric, &dto.Metric{
		Gauge: gDto,
	})
}

// NewGauge emits a new Gauge from the provided GaugeDesc descriptor.
func NewGauge(desc GaugeDesc) Gauge {
	return desc.build()
}

// GaugeVec proxies vector values, whereby metrics fan out per the provided
// label dimensions.
type GaugeVec interface {
	Metric

	// Set assigns the value of this GaugeVec for the provided label values.
	// The labels are provided as positional arguments and must match the
	// order defined in GaugeDesc.Labels.
	Set(float64, ...string)
	// Del deletes a given label set from this GaugeVec, indicating whether the
	// label set was deleted.
	Del(...string) bool
}

// GaugeVecDesc is the descriptor for a vector GaugeVec.
type GaugeVecDesc struct {
	Desc

	Labels []string // XXX
}

func (d *GaugeVecDesc) build() GaugeVec {
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

	return &gaugeVec{
		desc:     *d,
		children: make(map[uint64]*gaugeVecElem),
	}
}

type gaugeVecElem struct {
	mtx  sync.RWMutex
	val  float64
	dims []string
	desc GaugeVecDesc
}

func (g *gaugeVecElem) Set(v float64) {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	g.val = v
}

func (g *gaugeVecElem) Get() float64 {
	g.mtx.RLock()
	defer g.mtx.RUnlock()

	return g.val
}

func (g *gaugeVecElem) Write(o *dto.Metric) {
	o.Gauge = &dto.Gauge{
		Value: proto.Float64(g.Get()),
	}

	dims := make([]*dto.LabelPair, 0, len(g.desc.Labels))
	for i, n := range g.desc.Labels {
		dims = append(dims, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(g.dims[i]),
		})
	}
	o.Label = dims
}

type gaugeVec struct {
	GaugeVec

	desc     GaugeVecDesc
	mtx      sync.RWMutex
	children map[uint64]*gaugeVecElem
}

func (g *gaugeVec) Desc() Desc {
	return g.desc.Desc
}

func (g *gaugeVec) Write(out *dto.MetricFamily) {
	out.Type = dto.MetricType_GAUGE.Enum()

	g.mtx.RLock()
	gs := make([]*dto.Metric, 0, len(g.children))
	for _, e := range g.children {
		c := new(dto.Metric)
		e.Write(c)
		gs = append(gs, c)
	}
	g.mtx.RUnlock()

	out.Metric = gs
}

func (g *gaugeVec) Set(v float64, ls ...string) {
	if len(ls) != len(g.desc.Labels) || len(ls) == 0 {
		panic(errInconsistentCardinality)
	}

	h := hashLabelValues(ls...)

	g.mtx.RLock()
	if vec, ok := g.children[h]; ok {
		g.mtx.RUnlock()
		vec.Set(v)
		return
	}
	g.mtx.RUnlock()

	g.mtx.Lock()
	defer g.mtx.Unlock()
	if vec, ok := g.children[h]; ok {
		vec.Set(v)
		return
	}
	g.children[h] = &gaugeVecElem{
		val:  v,
		dims: ls,
		desc: g.desc,
	}
}

func (g *gaugeVec) Del(ls ...string) {
	if len(ls) != len(g.desc.Labels) || len(ls) == 0 {
		panic(errInconsistentCardinality)
	}

	h := hashLabelValues(ls...)

	h.mtx.Lock()
	defer h.mtx.Unlock()

	_, has := g.children[h]
	if !has {
		return false
	}
	del(g.children, h)
	return true
}

// NewGaugeVec emits a new GaugeVec from the provided GaugeVecDesc descriptor.
func NewGaugeVec(desc GaugeVecDesc) GaugeVec {
	return desc.build()
}
