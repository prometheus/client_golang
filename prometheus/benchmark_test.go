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
	"testing"
)

func BenchmarkCounter(b *testing.B) {
	type fns []func(*CounterVec) Counter

	twoConstraint := func(_ string) string {
		return "two"
	}

	deLV := func(m *CounterVec) Counter {
		return m.WithLabelValues("eins", "zwei", "drei")
	}
	frLV := func(m *CounterVec) Counter {
		return m.WithLabelValues("une", "deux", "trois")
	}
	nlLV := func(m *CounterVec) Counter {
		return m.WithLabelValues("een", "twee", "drie")
	}

	deML := func(m *CounterVec) Counter {
		return m.With(Labels{"two": "zwei", "one": "eins", "three": "drei"})
	}
	frML := func(m *CounterVec) Counter {
		return m.With(Labels{"two": "deux", "one": "une", "three": "trois"})
	}
	nlML := func(m *CounterVec) Counter {
		return m.With(Labels{"two": "twee", "one": "een", "three": "drie"})
	}

	deLabels := Labels{"two": "zwei", "one": "eins", "three": "drei"}
	dePML := func(m *CounterVec) Counter {
		return m.With(deLabels)
	}
	frLabels := Labels{"two": "deux", "one": "une", "three": "trois"}
	frPML := func(m *CounterVec) Counter {
		return m.With(frLabels)
	}
	nlLabels := Labels{"two": "twee", "one": "een", "three": "drie"}
	nlPML := func(m *CounterVec) Counter {
		return m.With(nlLabels)
	}

	table := []struct {
		name       string
		constraint LabelConstraint
		counters   fns
	}{
		{"labels=values,constraint=no", nil, fns{deLV}},
		{"labels=values,constraint=yes", twoConstraint, fns{deLV}},
		{"labels=values-triple,constraint=no", nil, fns{deLV, frLV, nlLV}},
		{"labels=values-triple,constraint=yes", twoConstraint, fns{deLV, frLV, nlLV}},
		{"labels=values-repeated,constraint=no", nil, fns{deLV, deLV}},
		{"labels=values-repeated,constraint=yes", twoConstraint, fns{deLV, deLV}},
		{"labels=mapped,constraint=no", nil, fns{deML}},
		{"labels=mapped,constraint=yes", twoConstraint, fns{deML}},
		{"labels=mapped-triple,constraint=no", nil, fns{deML, frML, nlML}},
		{"labels=mapped-triple,constraint=yes", twoConstraint, fns{deML, frML, nlML}},
		{"labels=mapped-repeated,constraint=no", nil, fns{deML, deML}},
		{"labels=mapped-repeated,constraint=yes", twoConstraint, fns{deML, deML}},
		{"labels=mapped-prepared,constraint=no", nil, fns{dePML}},
		{"labels=mapped-prepared,constraint=yes", twoConstraint, fns{dePML}},
		{"labels=mapped-prepared-triple,constraint=no", nil, fns{dePML, frPML, nlPML}},
		{"labels=mapped-prepared-triple,constraint=yes", twoConstraint, fns{dePML, frPML, nlPML}},
		{"labels=mapped-prepared-repeated,constraint=no", nil, fns{dePML, dePML}},
		{"labels=mapped-prepared-repeated,constraint=yes", twoConstraint, fns{dePML, dePML}},
	}

	for _, t := range table {
		b.Run(t.name, func(b *testing.B) {
			m := V2.NewCounterVec(
				CounterVecOpts{
					CounterOpts: CounterOpts{
						Name: "benchmark_counter",
						Help: "A counter to benchmark it.",
					},
					VariableLabels: ConstrainedLabels{
						ConstrainedLabel{Name: "one"},
						ConstrainedLabel{Name: "two", Constraint: t.constraint},
						ConstrainedLabel{Name: "three"},
					},
				},
			)
			b.ReportAllocs()
			for b.Loop() {
				for _, fn := range t.counters {
					fn(m).Inc()
				}
			}
		})
	}
}

func BenchmarkCounterWithLabelValuesConcurrent(b *testing.B) {
	m := NewCounterVec(
		CounterOpts{
			Name: "benchmark_counter",
			Help: "A counter to benchmark it.",
		},
		[]string{"one", "two", "three"},
	)
	b.ReportAllocs()
	b.ResetTimer()
	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < b.N/10; j++ {
				m.WithLabelValues("eins", "zwei", "drei").Inc()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkCounterNoLabels(b *testing.B) {
	m := NewCounter(CounterOpts{
		Name: "benchmark_counter",
		Help: "A counter to benchmark it.",
	})
	b.ReportAllocs()
	for b.Loop() {
		m.Inc()
	}
}

func BenchmarkGaugeWithLabelValues(b *testing.B) {
	m := NewGaugeVec(
		GaugeOpts{
			Name: "benchmark_gauge",
			Help: "A gauge to benchmark it.",
		},
		[]string{"one", "two", "three"},
	)
	b.ReportAllocs()
	for b.Loop() {
		m.WithLabelValues("eins", "zwei", "drei").Set(3.1415)
	}
}

func BenchmarkGaugeNoLabels(b *testing.B) {
	m := NewGauge(GaugeOpts{
		Name: "benchmark_gauge",
		Help: "A gauge to benchmark it.",
	})
	b.ReportAllocs()
	for b.Loop() {
		m.Set(3.1415)
	}
}

func BenchmarkSummaryWithLabelValues(b *testing.B) {
	m := NewSummaryVec(
		SummaryOpts{
			Name:       "benchmark_summary",
			Help:       "A summary to benchmark it.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"one", "two", "three"},
	)
	b.ReportAllocs()
	for b.Loop() {
		m.WithLabelValues("eins", "zwei", "drei").Observe(3.1415)
	}
}

func BenchmarkSummaryNoLabels(b *testing.B) {
	m := NewSummary(SummaryOpts{
		Name:       "benchmark_summary",
		Help:       "A summary to benchmark it.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	},
	)
	b.ReportAllocs()
	for b.Loop() {
		m.Observe(3.1415)
	}
}

func BenchmarkHistogramWithLabelValues(b *testing.B) {
	m := NewHistogramVec(
		HistogramOpts{
			Name: "benchmark_histogram",
			Help: "A histogram to benchmark it.",
		},
		[]string{"one", "two", "three"},
	)
	b.ReportAllocs()
	for b.Loop() {
		m.WithLabelValues("eins", "zwei", "drei").Observe(3.1415)
	}
}

func BenchmarkHistogramNoLabels(b *testing.B) {
	m := NewHistogram(HistogramOpts{
		Name: "benchmark_histogram",
		Help: "A histogram to benchmark it.",
	},
	)
	b.ReportAllocs()
	for b.Loop() {
		m.Observe(3.1415)
	}
}

func BenchmarkParallelCounter(b *testing.B) {
	c := NewCounter(CounterOpts{
		Name: "benchmark_counter",
		Help: "A Counter to benchmark it.",
	})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Inc()
		}
	})
}
