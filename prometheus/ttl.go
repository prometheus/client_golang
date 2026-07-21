// Copyright 2026 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus

import (
	"sync/atomic"
	"time"
)

// ExpiredCleaner is implemented by collectors that support TTL-based cleanup of
// unused children (for example MetricVec with a non-zero Opts.TTL).
//
// Registry.Gather only invokes CleanupExpired on collectors that also report
// TTL as enabled (see ttlEnabled), so vectors with TTL == 0 are not touched on
// the Gather hot path.
type ExpiredCleaner interface {
	CleanupExpired() int
}

// ttlEnabledCollector is the Gather-time check for automatic TTL cleanup.
// ttlEnabled is unexported so only types in this package (e.g. *MetricVec and
// the built-in *Vec types) can opt into automatic cleanup.
type ttlEnabledCollector interface {
	ExpiredCleaner
	ttlEnabled() bool
}

// ttlMetric is implemented by decorator wrappers that track last access time.
type ttlMetric interface {
	Metric
	lastAccessed() int64
	touch()
}

func nowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

// --- Counter wrapper ---

type ttlCounter struct {
	Counter
	lastAccessedTs atomic.Int64
}

func newTTLCounter(c Counter) *ttlCounter {
	tc := &ttlCounter{Counter: c}
	tc.lastAccessedTs.Store(nowUnixMilli())
	return tc
}

func (c *ttlCounter) Inc() {
	c.Counter.Inc()
	c.lastAccessedTs.Store(nowUnixMilli())
}

func (c *ttlCounter) Add(v float64) {
	c.Counter.Add(v)
	c.lastAccessedTs.Store(nowUnixMilli())
}

func (c *ttlCounter) AddWithExemplar(v float64, e Labels) {
	if ea, ok := c.Counter.(ExemplarAdder); ok {
		ea.AddWithExemplar(v, e)
	} else {
		c.Counter.Add(v)
	}
	c.lastAccessedTs.Store(nowUnixMilli())
}

func (c *ttlCounter) lastAccessed() int64 { return c.lastAccessedTs.Load() }
func (c *ttlCounter) touch()              { c.lastAccessedTs.Store(nowUnixMilli()) }

// --- Gauge wrapper ---

type ttlGauge struct {
	Gauge
	lastAccessedTs atomic.Int64
}

func newTTLGauge(g Gauge) *ttlGauge {
	tg := &ttlGauge{Gauge: g}
	tg.lastAccessedTs.Store(nowUnixMilli())
	return tg
}

func (g *ttlGauge) Set(v float64) {
	g.Gauge.Set(v)
	g.lastAccessedTs.Store(nowUnixMilli())
}

func (g *ttlGauge) Inc() {
	g.Gauge.Inc()
	g.lastAccessedTs.Store(nowUnixMilli())
}

func (g *ttlGauge) Dec() {
	g.Gauge.Dec()
	g.lastAccessedTs.Store(nowUnixMilli())
}

func (g *ttlGauge) Add(v float64) {
	g.Gauge.Add(v)
	g.lastAccessedTs.Store(nowUnixMilli())
}

func (g *ttlGauge) Sub(v float64) {
	g.Gauge.Sub(v)
	g.lastAccessedTs.Store(nowUnixMilli())
}

func (g *ttlGauge) SetToCurrentTime() {
	g.Gauge.SetToCurrentTime()
	g.lastAccessedTs.Store(nowUnixMilli())
}

func (g *ttlGauge) lastAccessed() int64 { return g.lastAccessedTs.Load() }
func (g *ttlGauge) touch()              { g.lastAccessedTs.Store(nowUnixMilli()) }

// --- Histogram wrapper ---

type ttlHistogram struct {
	Histogram
	lastAccessedTs atomic.Int64
}

func newTTLHistogram(h Histogram) *ttlHistogram {
	th := &ttlHistogram{Histogram: h}
	th.lastAccessedTs.Store(nowUnixMilli())
	return th
}

func (h *ttlHistogram) Observe(v float64) {
	h.Histogram.Observe(v)
	h.lastAccessedTs.Store(nowUnixMilli())
}

func (h *ttlHistogram) ObserveWithExemplar(v float64, e Labels) {
	if eo, ok := h.Histogram.(ExemplarObserver); ok {
		eo.ObserveWithExemplar(v, e)
	} else {
		h.Histogram.Observe(v)
	}
	h.lastAccessedTs.Store(nowUnixMilli())
}

func (h *ttlHistogram) lastAccessed() int64 { return h.lastAccessedTs.Load() }
func (h *ttlHistogram) touch()              { h.lastAccessedTs.Store(nowUnixMilli()) }

// --- Summary wrapper ---

type ttlSummary struct {
	Summary
	lastAccessedTs atomic.Int64
}

func newTTLSummary(s Summary) *ttlSummary {
	ts := &ttlSummary{Summary: s}
	ts.lastAccessedTs.Store(nowUnixMilli())
	return ts
}

func (s *ttlSummary) Observe(v float64) {
	s.Summary.Observe(v)
	s.lastAccessedTs.Store(nowUnixMilli())
}

func (s *ttlSummary) lastAccessed() int64 { return s.lastAccessedTs.Load() }
func (s *ttlSummary) touch()              { s.lastAccessedTs.Store(nowUnixMilli()) }
