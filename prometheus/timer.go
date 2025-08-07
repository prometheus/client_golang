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

import (
	"time"
)

// Timer is a helper type to time functions. Use NewTimer to create new
// instances.
type Timer struct {
	begin    time.Time
	observer Observer
}

// NewTimer creates a new Timer. The provided Observer is used to observe a
// duration in seconds. If the Observer implements ExemplarObserver, passing exemplar
// later on will be also supported.
// Timer is usually used to time a function call in the
// following way:
//
//	func TimeMe() {
//	    timer := NewTimer(myHistogram)
//	    defer timer.ObserveDuration()
//	    // Do actual work.
//	}
//
// or
//
//	func TimeMeWithExemplar() {
//		    timer := NewTimer(myHistogram)
//		    defer timer.ObserveDurationWithExemplar(exemplar)
//		    // Do actual work.
//		}
func NewTimer(o Observer) *Timer {
	return &Timer{
		begin:    time.Now(),
		observer: o,
	}
}

// ObserveDuration records the duration passed since the Timer was created with
// NewTimer. It calls the Observe method of the Observer provided during
// construction with the duration in seconds as an argument. The observed
// duration is also returned. ObserveDuration is usually called with a defer
// statement.
//
// Note that this method is only guaranteed to never observe negative durations
// if used with Go1.9+.
func (t *Timer) ObserveDuration() time.Duration {
	d := time.Since(t.begin)
	if t.observer != nil {
		t.observer.Observe(d.Seconds())
	}
	return d
}

// ObserveDurationWithExemplar is like ObserveDuration, but it will also
// observe exemplar with the duration unless exemplar is nil or provided Observer can't
// be casted to ExemplarObserver.
func (t *Timer) ObserveDurationWithExemplar(exemplar Labels) time.Duration {
	d := time.Since(t.begin)
	eo, ok := t.observer.(ExemplarObserver)
	if ok && exemplar != nil {
		eo.ObserveWithExemplar(d.Seconds(), exemplar)
		return d
	}
	if t.observer != nil {
		t.observer.Observe(d.Seconds())
	}
	return d
}

// TimerHistogram is a thin convenience wrapper around a Prometheus Histogram
// that makes it easier to time code paths in an idiomatic Go-style:
//
//	defer timer.Observe()()   // <-- starts the timer and defers the stop
type TimerHistogram struct {
	Histogram
}

func NewTimerHistogram(opts HistogramOpts) *TimerHistogram {
	t := &TimerHistogram{
		Histogram: NewHistogram(opts),
	}
	return t
}

// Observe starts a prom.Timer that records into the embedded Histogram and
// returns a “stop” callback.  Best used with defer:
//
//	defer timer.Observe()()
//
// The inner closure calls ObserveDuration on the hidden prom.Timer, recording
// the elapsed seconds into the histogram’s current bucket.
func (t *TimerHistogram) Observe() func() {
	timer := NewTimer(t.Histogram)
	return func() {
		timer.ObserveDuration()
	}
}

// Wrap executes fn() and records the time it took.  Equivalent to:
// Use when you don't need a defer chain (e.g., inside small helpers).
func (t *TimerHistogram) Wrap(fn func()) {
	defer t.Observe()()
	fn()
}

type TimerHistogramVec struct {
	*HistogramVec
}

func NewTimerHistogramVec(opts HistogramOpts, labelNames []string) *TimerHistogramVec {
	t := &TimerHistogramVec{
		HistogramVec: NewHistogramVec(opts, labelNames),
	}

	return t
}

// Observe return func for stop timer and observe value
// Example
// defer metric.Observe(map[string]string{"foo": "bar"})()
func (t *TimerHistogramVec) Observe(labels map[string]string) func() {
	timeStart := time.Now()
	return func() {
		d := time.Since(timeStart)
		t.HistogramVec.With(labels).Observe(d.Seconds())
	}
}

func (t *TimerHistogramVec) ObserveLabelValues(values ...string) func() {
	timeStart := time.Now()
	return func() {
		d := time.Since(timeStart)
		t.HistogramVec.WithLabelValues(values...).Observe(d.Seconds())
	}
}

func (t *TimerHistogramVec) Wrap(labels map[string]string, fn func()) {
	defer t.Observe(labels)()
	fn()
}

func (t *TimerHistogramVec) WrapLV(values []string, fn func()) {
	defer t.ObserveLabelValues(values...)()
	fn()
}

// TimerCounter is a minimal helper that turns a Prometheus **Counter** into a
// “stop-watch” for wall-clock time.
//
// Each call to Observe() starts a timer and, when the returned closure is
// executed, adds the elapsed seconds to the embedded Counter.  The counter
// therefore represents **the cumulative running time** across many code paths
// (e.g. total time spent processing all requests since process start).
//
// Compared with a Histogram-based timer you gain:
//
//   - A single monotonically-increasing number that is cheap to aggregate or
//     alert on (e.g. “CPU-seconds spent in GC”).
//   - Zero bucket management or percentile math.
//
// But you lose per-request latency data, so use it when you care about total
// time rather than distribution.
type TimerCounter struct {
	Counter
}

func NewTimerCounter(opts Opts) *TimerCounter {
	t := &TimerCounter{
		Counter: NewCounter(CounterOpts(opts)),
	}

	return t
}

// Observe starts a wall-clock timer and returns a “stop” closure.
//
// Typical usage:
//
//	defer myCounter.Observe()()   // records on function exit
//
// When the closure is executed it records the elapsed duration (in seconds)
// into the Counter.  Thread-safe as long as the underlying Counter is
// thread-safe (Prometheus counters are).
func (t *TimerCounter) Observe() func() {
	start := time.Now()

	return func() {
		d := time.Since(start)
		t.Counter.Add(d.Seconds())
	}
}

func (t *TimerCounter) Wrap(fn func()) {
	defer t.Observe()()
	fn()
}

type TimerCounterVec struct {
	*CounterVec
}

func NewTimerCounterVec(opts Opts, labels []string) *TimerCounterVec {
	t := &TimerCounterVec{
		CounterVec: NewCounterVec(CounterOpts(opts), labels),
	}

	return t
}

func (t *TimerCounterVec) Observe(labels map[string]string) func() {
	start := time.Now()

	return func() {
		d := time.Since(start)
		t.CounterVec.With(labels).Add(d.Seconds())
	}
}

func (t *TimerCounterVec) ObserveLabelValues(values ...string) func() {
	start := time.Now()
	return func() {
		d := time.Since(start)
		t.CounterVec.WithLabelValues(values...).Add(d.Seconds())
	}
}

func (t *TimerCounterVec) Wrap(labels map[string]string, fn func()) {
	defer t.Observe(labels)()
	fn()
}

func (t *TimerCounterVec) WrapLabelValues(values []string, fn func()) {
	defer t.ObserveLabelValues(values...)()
	fn()
}

// TimerContinuous is a variant of the standard Timer that **continuously updates**
// its underlying Counter while it is running—by default once every second—
// instead of emitting a single measurement only when the timer stops.
//
// Trade-offs
// ----------
//   - **Higher overhead** than a one-shot Timer (extra goroutine + ticker).
//   - **Finer-grained metrics** that are invaluable for long-running or
//     indeterminate-length activities such as stream processing, background
//     jobs, or large file transfers.
//   - **Sensitive to clock skew**—if the system clock is moved **backwards**
//     while the timer is running, the negative delta is silently discarded
//     (no panic), so that slice of time is lost from the measurement.
type TimerContinuous struct {
	Counter
	updateInterval time.Duration
}

func NewTimerContinuous(opts Opts, updateInterval time.Duration) *TimerContinuous {
	t := &TimerContinuous{
		Counter:        NewCounter(CounterOpts(opts)),
		updateInterval: updateInterval,
	}
	if t.updateInterval == 0 {
		t.updateInterval = time.Second
	}

	return t
}

func (t *TimerContinuous) Observe() func() {
	start := time.Now()
	ch := make(chan struct{})

	go func() {
		added := float64(0)
		ticker := time.NewTicker(t.updateInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ch:
				d := time.Since(start)
				if diff := d.Seconds() - added; diff > 0 {
					t.Counter.Add(diff)
				}
				return
			case <-ticker.C:
				d := time.Since(start)
				if diff := d.Seconds() - added; diff > 0 {
					t.Counter.Add(diff)
					added += diff
				}
			}
		}
	}()

	return func() {
		ch <- struct{}{}
	}
}

func (t *TimerContinuous) Wrap(fn func()) {
	defer t.Observe()()
	fn()
}

type TimerContinuousVec struct {
	*CounterVec
}

func NewTimerContinuousVec(opts Opts, labels []string) *TimerCounterVec {
	t := &TimerCounterVec{
		CounterVec: NewCounterVec(CounterOpts(opts), labels),
	}

	return t
}

func (t *TimerContinuousVec) Observe(labels map[string]string) func() {
	start := time.Now()
	ch := make(chan struct{})

	go func() {
		added := float64(0)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ch:
				d := time.Since(start)
				if diff := d.Seconds() - added; diff > 0 {
					t.CounterVec.With(labels).Add(diff)
				}
				return
			case <-ticker.C:
				d := time.Since(start)
				if diff := d.Seconds() - added; diff > 0 {
					t.CounterVec.With(labels).Add(diff)
					added += diff
				}
			}
		}
	}()

	return func() {
		ch <- struct{}{}
	}
}

func (t *TimerContinuousVec) ObserveLabelValues(values ...string) func() {
	start := time.Now()
	ch := make(chan struct{})

	go func() {
		added := float64(0)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ch:
				d := time.Since(start)
				if diff := d.Seconds() - added; diff > 0 {
					t.CounterVec.WithLabelValues(values...).Add(diff)
				}
				return
			case <-ticker.C:
				d := time.Since(start)
				if diff := d.Seconds() - added; diff > 0 {
					t.CounterVec.WithLabelValues(values...).Add(diff)
					added += diff
				}
			}
		}
	}()

	return func() {
		ch <- struct{}{}
	}
}

func (t *TimerContinuousVec) Wrap(labels map[string]string, fn func()) {
	defer t.Observe(labels)()
	fn()
}

func (t *TimerContinuousVec) WrapLabelValues(values []string, fn func()) {
	defer t.ObserveLabelValues(values...)()
	fn()
}
