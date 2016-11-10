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

type Timer struct {
	begin    time.Time
	observer Observer
	gauge    Gauge
}

func StartTimer() *Timer {
	return &Timer{begin: time.Now()}
}

func (t *Timer) With(o Observer) *Timer {
	t.observer = o
	return t
}

func (t *Timer) WithGauge(g Gauge) *Timer {
	t.gauge = g
	return t
}

func (t *Timer) Stop() {
	if t.observer != nil {
		t.observer.Observe(time.Since(t.begin).Seconds())
	}
	if t.gauge != nil {
		t.gauge.Set(time.Since(t.begin).Seconds())
	}
}
