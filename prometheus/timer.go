// Copyright 2016 The Prometheus Authors
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

import "time"

// Observer is the interface that wraps the Observe method, used by Histogram
// and Summary to add observations.
type Observer interface {
	Observe(float64)
}

// observerFunc is a function that implements Observer.
type observerFunc func(float64)

func (o observerFunc) Observe(value float64) {
	o(value)
}

// Timer is a helper type to time functions. Use NewTimer to create new
// instances. The most common usage pattern is to time a function with a
// Histogram in the following way:
//    func timeMe() {
//        t := NewTimer().With(myHistogram)
//        defer t.ObserveDuration()
//        // Do something.
//    }
// See the documentation of the methods for special cases.
type Timer struct {
	begin    time.Time
	observer Observer
}

// NewTimer creates a new Timer. To time anything, an Observer (i.e. a Histogram
// or a Summary) or a Gague has to be bound to it, the former with the With
// method, the latter with the WithGauge method.
func NewTimer() *Timer {
	return &Timer{begin: time.Now()}
}

// With binds an Observer (i.e. a Histogram or a Summary) to the Timer and
// returns the Timer for convenience. In most cases, the Observer to be used is
// already known at construction time, so it can be set immediately:
//     t := NewTimer().With(myObserver)
//
// With can also be called later, to bind an Observer for the first time or to
// change the bound Observer. It can be called with nil to unbind a previously
// bound Observer. Both is useful for recording with different Observers (or
// none) depending on the outcome of the timed function. Note that timing
// depending on the outcome can be confusing and should only be used after
// careful consideration.
//
// The ObserveDuration method of the Timer records the duration by calling the
// Observe method of the bound Observer with the time elapsed since NewTimer was
// called (in seconds).
//
// Note that a Gauge bound with WithGauge overrides the Observer bound with With
// and vice versa.
func (t *Timer) With(o Observer) *Timer {
	t.observer = o
	return t
}

// WithGauge binds a Gauge to the Timer and returns the Timer for
// convenience. This works in the same way as With works for Observers. To
// record the time, the Set method of the Gauge is called (with the recorded
// duration in seconds). Note that timing with Gauges is only useful for events
// that happen less frequently than the scrape interval or for one-shot batch
// jobs (where the recorded duration is pushed to a Pushgateway).
//
// Note that a Gauge bound with WithGauge overrides the Observer bound with With
// and vice versa.
func (t *Timer) WithGauge(g Gauge) *Timer {
	t.observer = observerFunc(g.Set)
	return t
}

// ObserveDuration records the duration passed since NewTimer was called. If an
// Observer has been bound with the With method, ObserveDuration calls its
// Observe method with the duration in seconds as an argument. If a Gauge has
// been bound with the WithGauge method, ObserveDuration calls its Set method
// with the duration in seconds as an argument. ObserveDuration is usually
// called with a defer statement.
func (t *Timer) ObserveDuration() {
	if t.observer != nil {
		t.observer.Observe(time.Since(t.begin).Seconds())
	}
}
