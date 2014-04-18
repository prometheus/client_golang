// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"errors"
	"sync"
	"time"

	"code.google.com/p/goprotobuf/proto"
	"github.com/streadway/quantile"

	dto "github.com/prometheus/client_model/go"
)

// XXX: Timer for summary.
// XXX: Standard http.HandlerFunc instrumentation pipeline.

// Summary captures individual observations from an event or sample stream and
// summarizes them in a manner similar to traditional summary statistics:
// 1. sum of observations, 2. observation count, 3. rank estimations.
type Summary interface {
	Metric

	Observe(v float64)
}

// DefObjectives are the default Summary quantile values and their respective
// levels of precision.  These should be suitable for most industrial purposes.
var DefObjectives = map[float64]float64{
	0.5:  0.05,
	0.90: 0.01,
	0.99: 0.001,
}

const (
	// DefFlush is the default flush interval for Summary metrics.
	DefFlush time.Duration = 15 * time.Minute
	// NoFlush indicates that a Summary should never flush its metrics.
	NoFlush = -1
)

// DefBufCap is the standard buffer size for collecting Summary observations.
const DefBufCap = 1024

// SummaryDesc is the descriptor for a scalar Summary.
type SummaryDesc struct {
	Desc

	// Objectives defines the quantile rank estimates with the tolerated level of
	// error defined as the value.  The default value is DefObjectives.
	Objectives map[float64]float64

	// FlushInter sets the interval at which the summary's event stream samples
	// are flushed.  This provides a stronger guarantee that stale data won't
	// crowd out more recent samples.  The default value is DefFlush.
	FlushInter time.Duration

	// BufCap defines the default sample stream buffer size.  The default value of
	// DefBufCap should suffice for most uses.
	BufCap int
}

func (d SummaryDesc) build() Summary {
	cap := d.BufCap
	if cap < 0 {
		panic("illegal capacity") // XXX
	} else if cap == 0 {
		cap = DefBufCap
	}

	if d.FlushInter == NoFlush {
		d.FlushInter = 0
	} else if d.FlushInter == 0 {
		d.FlushInter = DefFlush
	}

	if len(d.Objectives) == 0 {
		d.Objectives = DefObjectives
	}

	invs := make([]quantile.Estimate, 0, len(d.Objectives))
	for rank, acc := range d.Objectives {
		invs = append(invs, quantile.Known(rank, acc))
	}
	return &summary{
		desc:      d,
		hotBuf:    make([]float64, 0, cap),
		coldBuf:   make([]float64, 0, cap),
		lastFlush: time.Now(),
		invs:      invs,
	}
}

type summary struct {
	bufMtx sync.Mutex
	mtx    sync.Mutex

	desc            SummaryDesc
	sum             float64
	cnt             uint64
	hotBuf, coldBuf []float64

	invs []quantile.Estimate

	est *quantile.Estimator

	lastFlush time.Time
}

func (s *summary) Desc() Desc {
	return s.desc.Desc
}

func (s *summary) newEst() {
	if s.est != nil {
		panic("illegal")
	}

	s.est = quantile.New(s.invs...)
}

func (s *summary) fastIngest(v float64) bool {
	s.hotBuf = append(s.hotBuf, v)

	return len(s.hotBuf) < cap(s.hotBuf)
}

func (s *summary) slowIngest() {
	s.mtx.Lock()
	s.hotBuf, s.coldBuf = s.coldBuf, s.hotBuf
	s.hotBuf = s.hotBuf[0:0]

	// Unblock the original goroutine that was responsible for the mutation that
	// triggered the compaction.  But hold onto the global non-buffer state mutex
	// until the operation finishes.
	go func() {
		s.partialCompact()
		s.mtx.Unlock()
	}()
}

func (s *summary) partialCompact() {
	if s.est == nil {
		s.newEst()
	}
	for _, v := range s.coldBuf {
		s.est.Add(v)
		s.cnt++
		s.sum += v
	}
	s.coldBuf = s.coldBuf[0:0]
}

func (s *summary) fullCompact() {
	s.partialCompact()
	for _, v := range s.hotBuf {
		s.est.Add(v)
		s.cnt++
		s.sum += v
	}
	s.hotBuf = s.hotBuf[0:0]
}

func (s *summary) needFullCompact() bool {
	return !(s.est == nil && len(s.hotBuf) == 0)
}

func (s *summary) maybeFlush() {
	if s.desc.FlushInter == 0 {
		return
	}

	if time.Since(s.lastFlush) < s.desc.FlushInter {
		return
	}

	s.flush()
}

func (s *summary) flush() {
	s.est = nil
	s.lastFlush = time.Now()
}

func (s *summary) Observe(v float64) {
	s.bufMtx.Lock()
	defer s.bufMtx.Unlock()
	if ok := s.fastIngest(v); ok {
		return
	}

	s.slowIngest()
}

func (s *summary) Write(out *dto.MetricFamily) {
	if out == nil {
		panic("illegal")
	}
	out.Type = dto.MetricType_SUMMARY.Enum()

	s.bufMtx.Lock()
	s.mtx.Lock()

	sum := &dto.Summary{
		SampleCount: proto.Uint64(s.cnt),
		SampleSum:   proto.Float64(s.sum),
	}

	if s.needFullCompact() {
		s.fullCompact()
		qs := make([]*dto.Quantile, 0, len(s.desc.Objectives))
		for rank := range s.desc.Objectives {
			qs = append(qs, &dto.Quantile{
				Quantile: proto.Float64(rank),
				Value:    proto.Float64(s.est.Get(rank)),
			})
		}

		sum.Quantile = qs

	}

	s.maybeFlush()

	s.mtx.Unlock()
	s.bufMtx.Unlock()

	out.Metric = []*dto.Metric{{
		Summary: sum,
	}}
}

// NewSummary generates a new Summary from the provided descriptor.
func NewSummary(desc SummaryDesc) Summary {
	return desc.build()
}

// SummaryVec is exactly equivalent to Summary, except that it enables users to
// partition sample streams by unique label sets.
type SummaryVec interface {
	Metric

	Observe(float64, ...string)

	// XXX
	Del(...string) bool
}

// SummaryVecDesc describes a SummaryVec.
type SummaryVecDesc struct {
	Desc

	// Objectives defines the quantile rank estimates with the tolerated level of
	// error defined as the value.  The default value is DefObjectives.
	Objectives map[float64]float64

	// FlushInter sets the interval at which the summary's event stream samples
	// are flushed.  This provides a stronger guarantee that stale data won't
	// crowd out more recent samples.  The default value is DefFlush.
	FlushInter time.Duration

	// BufCap defines the default sample stream buffer size.  The default value of
	// DefBufCap should suffice for most uses.
	BufCap int

	// XXX
	Labels []string
}

var errIllegalCapDesc = errors.New("illegal buffer capacity")

func (d *SummaryVecDesc) build() SummaryVec {
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

	if d.BufCap < 0 {
		panic(errIllegalCapDesc)
	} else if d.BufCap == 0 {
		d.BufCap = DefBufCap
	}

	if d.FlushInter == NoFlush {
		d.FlushInter = 0
	} else if d.FlushInter == 0 {
		d.FlushInter = DefFlush
	}

	if len(d.Objectives) == 0 {
		d.Objectives = DefObjectives
	}

	invs := make([]quantile.Estimate, 0, len(d.Objectives))
	for rank, acc := range d.Objectives {
		invs = append(invs, quantile.Known(rank, acc))
	}

	return &summaryVec{
		d:        *d,
		children: make(map[uint64]*summaryVecElem),
		invs:     invs,
	}
}

type summaryVecElem struct {
	dims []string

	bufMtx sync.Mutex
	mtx    sync.Mutex

	desc            SummaryVecDesc
	sum             float64
	cnt             uint64
	hotBuf, coldBuf []float64

	invs []quantile.Estimate

	est *quantile.Estimator

	lastFlush time.Time
}

func (s *summaryVecElem) Observe(v float64) {
	s.bufMtx.Lock()
	defer s.bufMtx.Unlock()
	if ok := s.fastIngest(v); ok {
		return
	}

	s.slowIngest()
}

func (s *summaryVecElem) Write(o *dto.Metric) {
	s.bufMtx.Lock()
	s.mtx.Lock()

	o.Summary = &dto.Summary{
		SampleCount: proto.Uint64(s.cnt),
		SampleSum:   proto.Float64(s.sum),
	}

	if s.needFullCompact() {
		s.fullCompact()
		qs := make([]*dto.Quantile, 0, len(s.desc.Objectives))
		for rnk := range s.desc.Objectives {
			qs = append(qs, &dto.Quantile{
				Quantile: proto.Float64(rnk),
				Value:    proto.Float64(s.est.Get(rnk)),
			})
		}
		o.Summary.Quantile = qs
	}

	dims := make([]*dto.LabelPair, 0, len(s.desc.Labels))
	for i, n := range s.desc.Labels {
		dims = append(dims, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(s.dims[i]),
		})
	}
	o.Label = dims

	s.mtx.Unlock()
	s.bufMtx.Unlock()
}

func (s *summaryVecElem) newEst() {
	s.est = quantile.New(s.invs...)
}

func (s *summaryVecElem) fastIngest(v float64) bool {
	s.hotBuf = append(s.hotBuf, v)

	return len(s.hotBuf) < cap(s.hotBuf)
}

func (s *summaryVecElem) slowIngest() {
	s.mtx.Lock()
	s.hotBuf, s.coldBuf = s.coldBuf, s.hotBuf
	s.hotBuf = s.hotBuf[0:0]

	// Unblock the original goroutine that was responsible for the mutation that
	// triggered the compaction.  But hold onto the global non-buffer state mutex
	// until the operation finishes.
	go func() {
		s.partialCompact()
		s.mtx.Unlock()
	}()
}

func (s *summaryVecElem) partialCompact() {
	if s.est == nil {
		s.newEst()
	}
	for _, v := range s.coldBuf {
		s.est.Add(v)
		s.cnt++
		s.sum += v
	}
	s.coldBuf = s.coldBuf[0:0]
}

func (s *summaryVecElem) fullCompact() {
	s.partialCompact()
	for _, v := range s.hotBuf {
		s.est.Add(v)
		s.cnt++
		s.sum += v
	}
	s.hotBuf = s.hotBuf[0:0]
}

func (s *summaryVecElem) needFullCompact() bool {
	return !(s.est == nil && len(s.hotBuf) == 0)
}

func (s *summaryVecElem) maybeFlush() {
	if s.desc.FlushInter == 0 {
		return
	}

	if time.Since(s.lastFlush) < s.desc.FlushInter {
		return
	}

	s.flush()
}

func (s *summaryVecElem) flush() {
	s.est = nil
	s.lastFlush = time.Now()
}

type summaryVec struct {
	SummaryVec

	d        SummaryVecDesc
	m        sync.RWMutex
	invs     []quantile.Estimate
	children map[uint64]*summaryVecElem
}

func (s *summaryVec) Desc() Desc {
	return s.d.Desc
}

func (s *summaryVec) Write(out *dto.MetricFamily) {
	out.Type = dto.MetricType_SUMMARY.Enum()

	s.m.RLock()
	gs := make([]*dto.Metric, 0, len(s.children))
	for _, e := range s.children {
		c := new(dto.Metric)
		e.Write(c)
		gs = append(gs, c)
	}
	s.m.RUnlock()

	out.Metric = gs
}

func (s *summaryVec) Observe(v float64, dims ...string) {
	if len(dims) != len(s.d.Labels) || len(dims) == 0 {
		panic(errInconsistentCardinality)
	}

	h := hashLabelValues(dims...)

	s.m.RLock()
	if vec, ok := s.children[h]; ok {
		s.m.RUnlock()
		vec.Observe(v)
		return
	}
	s.m.RUnlock()

	s.m.Lock()
	defer s.m.Unlock()
	if vec, ok := s.children[h]; ok {
		vec.Observe(v)
		return
	}
	s.children[h] = &summaryVecElem{
		dims:      dims,
		hotBuf:    make([]float64, 0, s.d.BufCap),
		coldBuf:   make([]float64, 0, s.d.BufCap),
		lastFlush: time.Now(),
		invs:      s.invs,
	}
}

// NewSummaryVec generates a new SummaryVec from the provided descriptor.
func NewSummaryVec(desc SummaryVecDesc) SummaryVec {
	return desc.build()
}
