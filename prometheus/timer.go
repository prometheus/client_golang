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

type observerFunc func(float64)

func (o observerFunc) Observe(value float64) {
	o(value)
}

type Timer struct {
	begin    time.Time
	observer Observer
}

func NewTimer() *Timer {
	return &Timer{begin: time.Now()}
}

func (t *Timer) With(o Observer) *Timer {
	t.observer = o
	return t
}

func (t *Timer) WithGauge(g Gauge) *Timer {
	t.observer = observerFunc(g.Set)
	return t
}

func (t *Timer) ObserveDuration() {
	if t.observer != nil {
		t.observer.Observe(time.Since(t.begin).Seconds())
	}
}
