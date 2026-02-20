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
	"sync"
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

type TimerCounter struct {
	Counter
	updateInterval time.Duration
}

func NewTimerCounter(cnt Counter, updateInterval time.Duration) *TimerCounter {
	t := &TimerCounter{
		Counter:        cnt,
		updateInterval: updateInterval,
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
func (t *TimerCounter) Observe() (stop func()) {
	start := time.Now()
	ch := make(chan struct{})
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		added := float64(0)
		var updateChan <-chan time.Time
		if t.updateInterval > 0 {
			ticker := time.NewTicker(t.updateInterval)
			defer ticker.Stop()
			updateChan = ticker.C
		}
		for {
			select {
			case <-ch:
				d := time.Since(start)
				if diff := d.Seconds() - added; diff > 0 {
					t.Counter.Add(diff)
				}
				return
			case <-updateChan:
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
		wg.Wait()
	}
}

func (t *TimerCounter) Wrap(fn func()) {
	defer t.Observe()()
	fn()
}

func (t *TimerCounter) Add(dur time.Duration) {
	t.Counter.Add(dur.Seconds())
}

type TimerCounterVec struct {
	*CounterVec
	updateInterval time.Duration
}

func NewTimerCounterVec(cnt *CounterVec, updateInterval time.Duration) *TimerCounterVec {
	t := &TimerCounterVec{
		CounterVec:     cnt,
		updateInterval: updateInterval,
	}

	return t
}

func (t *TimerCounterVec) Observe(labels map[string]string) (stop func()) {
	start := time.Now()
	ch := make(chan struct{})
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		added := float64(0)
		var updateChan <-chan time.Time
		if t.updateInterval > 0 {
			ticker := time.NewTicker(t.updateInterval)
			defer ticker.Stop()
			updateChan = ticker.C
		}
		for {
			select {
			case <-ch:
				d := time.Since(start)
				if diff := d.Seconds() - added; diff > 0 {
					t.CounterVec.With(labels).Add(diff)
				}
				return
			case <-updateChan:
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
		wg.Wait()
	}
}

func (t *TimerCounterVec) ObserveLabelValues(values ...string) (stop func()) {
	start := time.Now()
	ch := make(chan struct{})
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		added := float64(0)
		var updateChan <-chan time.Time
		if t.updateInterval > 0 {
			ticker := time.NewTicker(t.updateInterval)
			defer ticker.Stop()
			updateChan = ticker.C
		}
		for {
			select {
			case <-ch:
				d := time.Since(start)
				if diff := d.Seconds() - added; diff > 0 {
					t.CounterVec.WithLabelValues(values...).Add(diff)
				}
				return
			case <-updateChan:
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
		wg.Wait()
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

func (t *TimerCounterVec) Add(dur time.Duration, labels map[string]string) {
	t.CounterVec.With(labels).Add(dur.Seconds())
}

func (t *TimerCounterVec) AddLabelValues(dur time.Duration, values ...string) {
	t.CounterVec.WithLabelValues(values...).Add(dur.Seconds())
}

type TimerObserver struct {
	Observer
}

func NewTimerObserver(obs Observer) *TimerObserver {
	t := &TimerObserver{
		Observer: obs,
	}

	return t
}

func (t *TimerObserver) Observe() (stop func()) {
	start := time.Now()
	return func() {
		d := time.Since(start)
		t.Observer.Observe(d.Seconds())
	}
}

func (t *TimerObserver) Wrap(fn func()) {
	defer t.Observe()()
	fn()
}

func (t *TimerObserver) Add(dur time.Duration) {
	t.Observer.Observe(dur.Seconds())
}

type TimerObserverVec struct {
	ObserverVec
}

func NewTimerObserverVec(obs ObserverVec) *TimerObserverVec {
	t := &TimerObserverVec{
		ObserverVec: obs,
	}

	return t
}

func (t *TimerObserverVec) Observe(labels map[string]string) func() {
	start := time.Now()
	return func() {
		d := time.Since(start)
		t.ObserverVec.With(labels).Observe(d.Seconds())
	}
}

func (t *TimerObserverVec) ObserveLabelValues(values ...string) func() {
	start := time.Now()
	return func() {
		d := time.Since(start)
		t.ObserverVec.WithLabelValues(values...).Observe(d.Seconds())
	}
}

func (t *TimerObserverVec) Wrap(labels map[string]string, fn func()) {
	defer t.Observe(labels)()
	fn()
}

func (t *TimerObserverVec) WrapLabelValues(values []string, fn func()) {
	defer t.ObserveLabelValues(values...)()
	fn()
}

func (t *TimerObserverVec) Add(dur time.Duration, labels map[string]string) {
	t.ObserverVec.With(labels).Observe(dur.Seconds())
}

func (t *TimerObserverVec) AddLabelValues(dur time.Duration, values ...string) {
	t.ObserverVec.WithLabelValues(values...).Observe(dur.Seconds())
}
