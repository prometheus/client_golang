// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"errors"
	"sort"
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
	MetricsCollector

	Observe(float64, ...string)
	Del(...string) bool
}

// DefObjectives are the default Summary quantile values and their respective
// levels of precision.  These should be suitable for most industrial purposes.
var (
	DefObjectives = map[float64]float64{
		0.5:  0.05,
		0.90: 0.01,
		0.99: 0.001,
	}
	errIllegalCapDesc = errors.New("illegal buffer capacity")
)

const (
	// DefFlush is the default flush interval for Summary metrics.
	DefFlush time.Duration = 15 * time.Minute
	// NoFlush indicates that a Summary should never flush its metrics.
	NoFlush = -1
)

// DefBufCap is the standard buffer size for collecting Summary observations.
const DefBufCap = 1024

// SummaryOptions determines options for a Summary.
type SummaryOptions struct {
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

// NewSummary generates a new Summary from the provided descriptor and options.
// The descriptor's Type field is ignored and forcefully set to MetricType_SUMMARY.
func NewSummary(desc *Desc, opts *SummaryOptions) Summary {
	desc.Type = dto.MetricType_SUMMARY

	if opts.BufCap < 0 {
		panic(errIllegalCapDesc)
	} else if opts.BufCap == 0 {
		opts.BufCap = DefBufCap
	}

	if opts.FlushInter == NoFlush {
		opts.FlushInter = 0
	} else if opts.FlushInter == 0 {
		opts.FlushInter = DefFlush
	}

	if len(opts.Objectives) == 0 {
		opts.Objectives = DefObjectives
	}

	invs := make([]quantile.Estimate, 0, len(opts.Objectives))
	for rank, acc := range opts.Objectives {
		invs = append(invs, quantile.Known(rank, acc))
	}

	if len(desc.VariableLabels) == 0 {
		result := &summary{
			desc:      desc,
			opts:      opts,
			hotBuf:    make([]float64, 0, opts.BufCap),
			coldBuf:   make([]float64, 0, opts.BufCap),
			lastFlush: time.Now(),
			invs:      invs,
		}
		result.Self = result
		return result
	}
	result := &summaryVec{
		desc:     desc,
		opts:     opts,
		children: make(map[uint64]*summaryVecElem),
		invs:     invs,
	}
	result.Self = result
	return result
}

type summary struct {
	SelfCollector

	bufMtx sync.Mutex
	mtx    sync.Mutex

	desc            *Desc
	opts            *SummaryOptions
	sum             float64
	cnt             uint64
	hotBuf, coldBuf []float64

	invs []quantile.Estimate

	est *quantile.Estimator

	lastFlush time.Time
}

func (s *summary) Desc() *Desc {
	return s.desc
}

func (s *summary) newEst() {
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
	if s.opts.FlushInter == 0 {
		return
	}

	if time.Since(s.lastFlush) < s.opts.FlushInter {
		return
	}

	s.flush()
}

func (s *summary) flush() {
	s.est = nil
	s.lastFlush = time.Now()
}

func (s *summary) Observe(v float64, dims ...string) {
	if len(dims) != 0 {
		panic(errInconsistentCardinality)
	}
	s.bufMtx.Lock()
	defer s.bufMtx.Unlock()
	if ok := s.fastIngest(v); ok {
		return
	}

	s.slowIngest()
}

func (s *summary) Del(dims ...string) bool {
	if len(dims) != 0 {
		panic(errInconsistentCardinality)
	}
	return false
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
		qs := make([]*dto.Quantile, 0, len(s.opts.Objectives))
		for rank := range s.opts.Objectives {
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
		Label:   s.desc.presetLabelPairs,
	}}
}

type summaryVecElem struct {
	dims []string

	bufMtx sync.Mutex
	mtx    sync.Mutex

	desc            *Desc
	opts            *SummaryOptions
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

	s.sum += v
	s.cnt++

	if ok := s.fastIngest(v); ok {
		return
	}

	s.slowIngest()
}

type quantSort []*dto.Quantile

func (s quantSort) Len() int {
	return len(s)
}

func (s quantSort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s quantSort) Less(i, j int) bool {
	return s[i].GetQuantile() < s[j].GetQuantile()
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
		qs := make([]*dto.Quantile, 0, len(s.opts.Objectives))
		for rnk := range s.opts.Objectives {
			qs = append(qs, &dto.Quantile{
				Quantile: proto.Float64(rnk),
				Value:    proto.Float64(s.est.Get(rnk)),
			})
		}
		o.Summary.Quantile = qs
	}

	s.mtx.Unlock()
	s.bufMtx.Unlock()

	dims := make([]*dto.LabelPair, 0, len(s.desc.PresetLabels)+len(s.desc.VariableLabels))
	dims = append(dims, s.desc.presetLabelPairs...)
	for i, n := range s.desc.VariableLabels {
		dims = append(dims, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(s.dims[i]),
		})
	}
	sort.Sort(lpSorter(dims))
	o.Label = dims

	if len(o.Summary.Quantile) > 0 {
		sort.Sort(quantSort(o.Summary.Quantile))
	}
	sort.Sort(lpSorter(dims))
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
	if s.opts.FlushInter == 0 {
		return
	}

	if time.Since(s.lastFlush) < s.opts.FlushInter {
		return
	}

	s.flush()
}

func (s *summaryVecElem) flush() {
	s.est = nil
	s.lastFlush = time.Now()
}

type summaryVec struct {
	SelfCollector

	desc     *Desc
	opts     *SummaryOptions
	mtx      sync.RWMutex
	invs     []quantile.Estimate
	children map[uint64]*summaryVecElem
}

func (s *summaryVec) Desc() *Desc {
	return s.desc
}

func (s *summaryVec) Write(out *dto.MetricFamily) {
	out.Type = dto.MetricType_SUMMARY.Enum()

	s.mtx.RLock()
	elems := map[uint64]*summaryVecElem{}
	hashes := make([]uint64, 0, len(elems))
	for h, e := range s.children {
		elems[h] = e
		hashes = append(hashes, h)
	}
	s.mtx.RUnlock()

	sort.Sort(hashSorter(hashes))

	ss := make([]*dto.Metric, 0, len(hashes))
	for _, h := range hashes {
		c := new(dto.Metric)
		elems[h].Write(c)
		ss = append(ss, c)
	}

	out.Metric = ss
}

func (s *summaryVec) Observe(v float64, dims ...string) {
	if len(dims) != len(s.desc.VariableLabels) {
		panic(errInconsistentCardinality)
	}

	h := hashLabelValues(dims...)

	s.mtx.RLock()
	if vec, ok := s.children[h]; ok {
		s.mtx.RUnlock()
		vec.Observe(v)
		return
	}
	s.mtx.RUnlock()

	s.mtx.Lock()
	defer s.mtx.Unlock()
	if vec, ok := s.children[h]; ok {
		vec.Observe(v)
		return
	}
	s.children[h] = &summaryVecElem{
		desc:      s.desc,
		opts:      s.opts,
		dims:      dims,
		hotBuf:    make([]float64, 0, s.opts.BufCap),
		coldBuf:   make([]float64, 0, s.opts.BufCap),
		lastFlush: time.Now(),
		invs:      s.invs,
	}
	s.children[h].Observe(v) // XXX
}

func (s *summaryVec) Del(ls ...string) bool {
	// TODO: Is this doing the right thing?
	if len(ls) != len(s.desc.VariableLabels) {
		panic(errInconsistentCardinality)
	}

	h := hashLabelValues(ls...)

	s.mtx.Lock()
	defer s.mtx.Unlock()

	if _, has := s.children[h]; !has {
		return false
	}
	delete(s.children, h)
	return true
}
