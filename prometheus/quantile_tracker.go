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
	"math"
)

// Tracker is the interface for single quantile trackers.
// Observe(v) to add new point v.
// Estimate() to return the current estimate.
type Tracker interface {
	Observe(v float64)
	Estimate() float64
}

// TrackerMaker is the interface for a tracker contstructor.
// Note, this is not particularly general, but three float64 parameters works
// for both kinds of tracker to be used here.
type TrackerMaker func(q, lam, rho, est float64) Tracker

// MTracker is the interface for a multiple quantile tracker.
// This only exists to allow constructing the same multiple quantile tracker
// with different inner trackers for evaluation.
// Observe to insert new data.
// Estimate to return a vector of the current estimates.
type MTracker interface {
	Observe(v float64)
	Estimate() []float64
}

// MSPITracker implementation inspired by OnlineStats.jl
type MSPITracker struct {
	est float64 // Current estimate
	q   float64 // Target quantile
	lam float64 // Estimate convergence rate parameter
	rho float64 // Small value for majorisation (epsilon in Julia version)
}

// Estimate returns the current estimate from a tracker.
func (t *MSPITracker) Estimate() float64 {
	return t.est
}

// Observe inserts new data.
func (t *MSPITracker) Observe(v float64) {
	vt := 1.0 / (math.Abs(v-t.est) + t.rho)
	t.est = (t.est + t.lam*(t.q-0.5+vt*v/2.0)) / (1.0 + t.lam*vt/2.0)
}

// NewMSPITracker constructs an MSPITracker.
func NewMSPITracker(q, lam, rho, est float64) Tracker {
	if (q < 0.0) || (q > 1.0) {
		return nil
	}
	r := MSPITracker{
		est: est,
		q:   q,
		lam: lam,
		rho: rho,
	}
	return &r
}

// QEMATracker is an Implementation of:
// Quantile Tracking in Dynamically Varying Data Streams Using a Generalized
// Exponentially Weighted Average of Observations
// Hammer, Yazidi, and Rue
// https://arxiv.org/pdf/1901.04681.pdf
type QEMATracker struct {
	est float64 // Current estimate
	q   float64 // Target quantile
	lam float64 // Estimate convergence rate parameter
	rho float64 // Conditional expectation convergence rate parameter
	ul  float64 // Low-side conditional expectation
	uh  float64 // High-side conditional expectation
}

// NewQEMATracker constructs a new tracker, using QEMATracker for the inner trackers..
func NewQEMATracker(q, lam, rho, est float64) Tracker {
	if (q < 0.0) || (q > 1.0) {
		return nil
	}
	r := QEMATracker{
		ul:  (est - 1) / 2,
		uh:  (est + 1) / 2,
		est: est,
		q:   q,
		lam: lam,
		rho: rho,
	}
	return &r
}

// Estimate returns the current estimate from a tracker.
func (t *QEMATracker) Estimate() float64 {
	return t.est
}

// Observe inserts new data.
func (t *QEMATracker) Observe(v float64) {
	if t.ul > t.uh {
		t.ul, t.uh = t.uh, t.ul
	}
	a := (t.q / (t.uh - t.est)) / (t.q/(t.uh-t.est) + (1-t.q)/(t.est-t.ul))
	var b float64
	if v < t.est {
		b = t.lam * (1 - a)
	} else {
		b = t.lam * a
	}
	nest := (1-b)*t.est + b*v
	if v > t.est {
		t.uh = nest - t.est + (1-t.rho)*t.uh + t.rho*v
		t.ul = nest - t.est + t.ul
	} else {
		t.uh = nest - t.est + t.uh
		t.ul = nest - t.est + (1-t.rho)*t.ul + t.rho*v
	}
	t.est = nest
}

// MQTracker is an implementation of:
// Joint Tracking of Multiple Quantiles Through Conditional Quantiles
// Hammer, Yazidi, and Rue
// https://arxiv.org/pdf/1902.05428.pdf
// This version allows substituting different internal trackers.
type MQTracker struct {
	est       []float64 // Current estimates
	tr        []Tracker // Individual trackers
	K         int       // length(quantiles)
	c         int       // Index of position quantile
	quantiles []float64 // Quantiles to track
	lam       float64   // Position convergence rate parameter
	gam       float64   // Shape convergence rate parameter
	rho       float64   // Conditional expectation convergence rate parameter
}

// Estimate returs a slice containing the current estimates.
// Note: for threading purposes, this is a copy, thus can be modified.
// However, the function itself is not threadsafe, so should be called under
// a lock.
func (t *MQTracker) Estimate() []float64 {
	est := append(t.est[:0:0], t.est...)
	return est
}

// Observe inserts new data.
// Note: not threadsafe, should be called under a lock.
func (t *MQTracker) Observe(v float64) {
	t.tr[t.c].Observe(v)
	t.est[t.c] = t.tr[t.c].Estimate()
	for k := t.c - 1; k >= 0; k-- {
		nest := t.est[k+1]
		if v < nest {
			y := v - nest
			t.tr[k].Observe(y)
		}
		t.est[k] = t.tr[k].Estimate() + nest
	}
	for k := t.c + 1; k < t.K; k++ {
		nest := t.est[k-1]
		if v > nest {
			y := v - nest
			t.tr[k].Observe(y)
		}
		t.est[k] = t.tr[k].Estimate() + nest
	}
}

// Generic constructor, not exported.
func newMTracker(maker TrackerMaker, q []float64, lam, gam, rho float64) *MQTracker {
	K := len(q)
	// Find the index of the quantile closest to the median.
	// This will be the one that tracks the position of the distribution.
	// Override after construction if necessary (can be a good idea if you
	// know the distribution has a mass at one end of the support, e.g.
	// use 0 if tracking latency).
	var c int
	var min = 2.0 // Deliberately crazy value.
	for i, v := range q {
		v2 := math.Abs(v - 0.5)
		if v2 < min {
			min = v2
			c = i
		}
	}
	r := MQTracker{
		make([]float64, len(q)),
		make([]Tracker, len(q)),
		K, c,
		make([]float64, len(q)),
		lam, gam, rho}
	for i := 0; i < K; i++ {
		r.est[i] = 0
	}
	r.quantiles[c] = q[c]
	for k := c - 1; k >= 0; k-- {
		r.quantiles[k] = q[k] / q[k+1]
	}
	for k := c + 1; k < K; k++ {
		r.quantiles[k] = (q[k] - q[k-1]) / (1 - q[k-1])
	}
	for i := 0; i < K; i++ {
		l := gam
		if i == c {
			l = lam
		}
		r.tr[i] = maker(r.quantiles[i], l, rho, r.est[i])
	}
	return &r
}

// NewMQEMATracker constructs a MQTracker instance using QEMA inner trackers.
func NewMQEMATracker(q []float64, lam, gam, rho float64) *MQTracker {
	return newMTracker(NewQEMATracker, q, lam, gam, rho)
}

// NewMMSPITracker constructs a MQTracker instance using MSPI inner trackers.
func NewMMSPITracker(q []float64, lam, gam, rho float64) *MQTracker {
	return newMTracker(NewMSPITracker, q, lam, gam, rho)
}
