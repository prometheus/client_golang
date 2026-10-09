// Copyright The Prometheus Authors
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
// unused children (for example MetricVec with a positive MetricVecOpts.TTL).
//
// Registry.Gather invokes CleanupExpired on registered collectors. Vectors with
// TTL <= 0 make CleanupExpired a no-op.
// CleanupExpired returns the number of children removed and must be safe to
// call concurrently with Collect and other CleanupExpired calls.
type ExpiredCleaner interface {
	CleanupExpired() int
}

// ttlMetric is implemented by decorator wrappers that track last access time.
type ttlMetric interface {
	Metric
	lastAccessed() int64
	touch()
}

type ttlMetricWrapper struct {
	Metric
	lastAccessedTs atomic.Int64
}

func newTTLMetric(metric Metric) *ttlMetricWrapper {
	tm := &ttlMetricWrapper{Metric: metric}
	tm.lastAccessedTs.Store(time.Now().UnixMilli())
	return tm
}

func (m *ttlMetricWrapper) lastAccessed() int64 { return m.lastAccessedTs.Load() }
func (m *ttlMetricWrapper) touch()              { m.lastAccessedTs.Store(time.Now().UnixMilli()) }

// unwrapTTLMetric preserves the concrete type returned by custom constructors.
func unwrapTTLMetric(metric Metric) Metric {
	if tm, ok := metric.(*ttlMetricWrapper); ok {
		return tm.Metric
	}
	return metric
}

// --- Counter wrapper ---

type ttlCounter struct {
	Counter
	lastAccessedTs atomic.Int64
}

func newTTLCounter(c Counter) *ttlCounter {
	tc := &ttlCounter{Counter: c}
	tc.lastAccessedTs.Store(time.Now().UnixMilli())
	return tc
}

func (c *ttlCounter) Inc() {
	c.Counter.Inc()
	c.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (c *ttlCounter) Add(v float64) {
	c.Counter.Add(v)
	c.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (c *ttlCounter) AddWithExemplar(v float64, e Labels) {
	if ea, ok := c.Counter.(ExemplarAdder); ok {
		ea.AddWithExemplar(v, e)
	} else {
		c.Counter.Add(v)
	}
	c.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (c *ttlCounter) lastAccessed() int64 { return c.lastAccessedTs.Load() }
func (c *ttlCounter) touch()              { c.lastAccessedTs.Store(time.Now().UnixMilli()) }

// --- Gauge wrapper ---

type ttlGauge struct {
	Gauge
	lastAccessedTs atomic.Int64
}

func newTTLGauge(g Gauge) *ttlGauge {
	tg := &ttlGauge{Gauge: g}
	tg.lastAccessedTs.Store(time.Now().UnixMilli())
	return tg
}

func (g *ttlGauge) Set(v float64) {
	g.Gauge.Set(v)
	g.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (g *ttlGauge) Inc() {
	g.Gauge.Inc()
	g.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (g *ttlGauge) Dec() {
	g.Gauge.Dec()
	g.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (g *ttlGauge) Add(v float64) {
	g.Gauge.Add(v)
	g.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (g *ttlGauge) Sub(v float64) {
	g.Gauge.Sub(v)
	g.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (g *ttlGauge) SetToCurrentTime() {
	g.Gauge.SetToCurrentTime()
	g.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (g *ttlGauge) lastAccessed() int64 { return g.lastAccessedTs.Load() }
func (g *ttlGauge) touch()              { g.lastAccessedTs.Store(time.Now().UnixMilli()) }

// --- Histogram wrapper ---

type ttlHistogram struct {
	Histogram
	lastAccessedTs atomic.Int64
}

func newTTLHistogram(h Histogram) *ttlHistogram {
	th := &ttlHistogram{Histogram: h}
	th.lastAccessedTs.Store(time.Now().UnixMilli())
	return th
}

func (h *ttlHistogram) Observe(v float64) {
	h.Histogram.Observe(v)
	h.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (h *ttlHistogram) ObserveWithExemplar(v float64, e Labels) {
	if eo, ok := h.Histogram.(ExemplarObserver); ok {
		eo.ObserveWithExemplar(v, e)
	} else {
		h.Histogram.Observe(v)
	}
	h.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (h *ttlHistogram) lastAccessed() int64 { return h.lastAccessedTs.Load() }
func (h *ttlHistogram) touch()              { h.lastAccessedTs.Store(time.Now().UnixMilli()) }

// --- Summary wrapper ---

type ttlSummary struct {
	Summary
	lastAccessedTs atomic.Int64
}

func newTTLSummary(s Summary) *ttlSummary {
	ts := &ttlSummary{Summary: s}
	ts.lastAccessedTs.Store(time.Now().UnixMilli())
	return ts
}

func (s *ttlSummary) Observe(v float64) {
	s.Summary.Observe(v)
	s.lastAccessedTs.Store(time.Now().UnixMilli())
}

func (s *ttlSummary) lastAccessed() int64 { return s.lastAccessedTs.Load() }
func (s *ttlSummary) touch()              { s.lastAccessedTs.Store(time.Now().UnixMilli()) }
