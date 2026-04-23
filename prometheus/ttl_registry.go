// Copyright 2014 The Prometheus Authors
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
	"sync"
	"time"

	dto "github.com/prometheus/client_model/go"
)

// TTLRegistry is a dedicated Prometheus registry for metrics that need
// time-to-live behavior on *Vec children. It embeds a plain *Registry and adds:
//   - Vec constructors that enable per-child TTL (same semantics as MetricVec
//     built with NewMetricVecWithTTL).
//   - Automatic CleanupExpired on all Vecs created through this registry before
//     each Gather, so memory can be reclaimed even when scrapes are infrequent
//     (see discussion in https://github.com/prometheus/client_golang/issues/1983).
//
// Default prometheus.NewRegistry, NewCounterVec, and Opts are unchanged; use
// TTLRegistry only when you explicitly opt in to Vec TTL.
type TTLRegistry struct {
	*Registry

	ttl time.Duration

	mu   sync.Mutex
	vecs []*MetricVec
}

// NewTTLRegistry returns a registry backed by a new empty *Registry. ttl must
// be greater than zero; it applies to every Vec created via this registry's
// constructor methods.
func NewTTLRegistry(ttl time.Duration) *TTLRegistry {
	if ttl <= 0 {
		panic("NewTTLRegistry: ttl must be > 0")
	}
	return &TTLRegistry{
		Registry: NewRegistry(),
		ttl:      ttl,
	}
}

func (r *TTLRegistry) track(mv *MetricVec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vecs = append(r.vecs, mv)
}

func (r *TTLRegistry) runCleanup() {
	r.mu.Lock()
	vecs := append([]*MetricVec(nil), r.vecs...)
	r.mu.Unlock()
	for _, mv := range vecs {
		mv.CleanupExpired()
	}
}

// Gather implements Gatherer. It runs CleanupExpired on all Vecs created
// through this TTLRegistry, then delegates to the embedded Registry.
func (r *TTLRegistry) Gather() ([]*dto.MetricFamily, error) {
	r.runCleanup()
	return r.Registry.Gather()
}

// NewCounterVec is like prometheus.NewCounterVec but enables Vec TTL using this
// registry's ttl, registers the Vec, and tracks it for Gather-time cleanup.
func (r *TTLRegistry) NewCounterVec(opts CounterOpts, labelNames []string) *CounterVec {
	return r.NewCounterVecOpts(CounterVecOpts{
		CounterOpts:    opts,
		VariableLabels: UnconstrainedLabels(labelNames),
	})
}

// NewCounterVecOpts is like V2.NewCounterVec with TTL and automatic registration.
func (r *TTLRegistry) NewCounterVecOpts(opts CounterVecOpts) *CounterVec {
	cv := newCounterVecWithTTL(opts, r.ttl)
	r.MustRegister(cv)
	r.track(cv.MetricVec)
	return cv
}

// NewGaugeVec is like prometheus.NewGaugeVec with TTL, registration, and tracking.
func (r *TTLRegistry) NewGaugeVec(opts GaugeOpts, labelNames []string) *GaugeVec {
	return r.NewGaugeVecOpts(GaugeVecOpts{
		GaugeOpts:      opts,
		VariableLabels: UnconstrainedLabels(labelNames),
	})
}

// NewGaugeVecOpts is like V2.NewGaugeVec with TTL and automatic registration.
func (r *TTLRegistry) NewGaugeVecOpts(opts GaugeVecOpts) *GaugeVec {
	gv := newGaugeVecWithTTL(opts, r.ttl)
	r.MustRegister(gv)
	r.track(gv.MetricVec)
	return gv
}

// NewHistogramVec is like prometheus.NewHistogramVec with TTL, registration, and tracking.
func (r *TTLRegistry) NewHistogramVec(opts HistogramOpts, labelNames []string) *HistogramVec {
	return r.NewHistogramVecOpts(HistogramVecOpts{
		HistogramOpts:  opts,
		VariableLabels: UnconstrainedLabels(labelNames),
	})
}

// NewHistogramVecOpts is like V2.NewHistogramVec with TTL and automatic registration.
func (r *TTLRegistry) NewHistogramVecOpts(opts HistogramVecOpts) *HistogramVec {
	hv := newHistogramVecWithTTL(opts, r.ttl)
	r.MustRegister(hv)
	r.track(hv.MetricVec)
	return hv
}
