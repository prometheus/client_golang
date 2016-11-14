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
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestTimerObserve(t *testing.T) {
	his := NewHistogram(HistogramOpts{
		Name: "test_histogram",
	})
	sum := NewSummary(SummaryOpts{
		Name: "test_summary",
	})
	gauge := NewGauge(GaugeOpts{
		Name: "test_gauge",
	})

	func() {
		hisTimer := NewTimer().With(his)
		sumTimer := NewTimer().With(sum)
		gaugeTimer := NewTimer().WithGauge(gauge)
		defer hisTimer.ObserveDuration()
		defer sumTimer.ObserveDuration()
		defer gaugeTimer.ObserveDuration()
	}()

	m := &dto.Metric{}
	his.Write(m)
	if want, got := uint64(1), m.GetHistogram().GetSampleCount(); want != got {
		t.Errorf("want %d observations for histogram, got %d", want, got)
	}
	m.Reset()
	sum.Write(m)
	if want, got := uint64(1), m.GetSummary().GetSampleCount(); want != got {
		t.Errorf("want %d observations for summary, got %d", want, got)
	}
	m.Reset()
	gauge.Write(m)
	if got := m.GetGauge().GetValue(); got <= 0 {
		t.Errorf("want value > 0 for gauge, got %f", got)
	}
}

func TestTimerEmpty(t *testing.T) {
	emptyTimer := NewTimer()
	emptyTimer.ObserveDuration()
	// Do nothing, just demonstrate it works without panic.
}

func TestTimerUnset(t *testing.T) {
	his := NewHistogram(HistogramOpts{
		Name: "test_histogram",
	})

	func() {
		timer := NewTimer().With(his)
		defer timer.ObserveDuration()
		timer.With(nil)
	}()

	m := &dto.Metric{}
	his.Write(m)
	if want, got := uint64(0), m.GetHistogram().GetSampleCount(); want != got {
		t.Errorf("want %d observations for histogram, got %d", want, got)
	}
}

func TestTimerChange(t *testing.T) {
	his := NewHistogram(HistogramOpts{
		Name: "test_histogram",
	})
	sum := NewSummary(SummaryOpts{
		Name: "test_summary",
	})

	func() {
		timer := NewTimer().With(his)
		defer timer.ObserveDuration()
		timer.With(sum)
	}()

	m := &dto.Metric{}
	his.Write(m)
	if want, got := uint64(0), m.GetHistogram().GetSampleCount(); want != got {
		t.Errorf("want %d observations for histogram, got %d", want, got)
	}
	m.Reset()
	sum.Write(m)
	if want, got := uint64(1), m.GetSummary().GetSampleCount(); want != got {
		t.Errorf("want %d observations for summary, got %d", want, got)
	}
}
