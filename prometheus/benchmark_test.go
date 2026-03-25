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
	"fmt"
	"runtime"
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
		{"With Label Values", nil, fns{deLV}},
		{"With Label Values and Constraint", twoConstraint, fns{deLV}},
		{"With triple Label Values", nil, fns{deLV, frLV, nlLV}},
		{"With triple Label Values and Constraint", twoConstraint, fns{deLV, frLV, nlLV}},
		{"With repeated Label Values", nil, fns{deLV, deLV}},
		{"With repeated Label Values and Constraint", twoConstraint, fns{deLV, deLV}},
		{"With Mapped Labels", nil, fns{deML}},
		{"With Mapped Labels and Constraint", twoConstraint, fns{deML}},
		{"With triple Mapped Labels", nil, fns{deML, frML, nlML}},
		{"With triple Mapped Labels and Constraint", twoConstraint, fns{deML, frML, nlML}},
		{"With repeated Mapped Labels", nil, fns{deML, deML}},
		{"With repeated Mapped Labels and Constraint", twoConstraint, fns{deML, deML}},
		{"With Prepared Mapped Labels", nil, fns{dePML}},
		{"With Prepared Mapped Labels and Constraint", twoConstraint, fns{dePML}},
		{"With triple Prepared Mapped Labels", nil, fns{dePML, frPML, nlPML}},
		{"With triple Prepared Mapped Labels and Constraint", twoConstraint, fns{dePML, frPML, nlPML}},
		{"With repeated Prepared Mapped Labels", nil, fns{dePML, dePML}},
		{"With repeated Prepared Mapped Labels and Constraint", twoConstraint, fns{dePML, dePML}},
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
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.WithLabelValues("eins", "zwei", "drei").Set(3.1415)
	}
}

func BenchmarkGaugeNoLabels(b *testing.B) {
	m := NewGauge(GaugeOpts{
		Name: "benchmark_gauge",
		Help: "A gauge to benchmark it.",
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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

// doWork simulates realistic application work between metric operations.
// 10,000 iterations of integer addition approximates a few microseconds of CPU
// work, matching the pattern from the regression report in #1748 where a cache
// lookup or short computation precedes each metric update.
// The v parameter accepts the loop index to prevent compile-time constant folding.
func doWork(v int) int {
	for i := 0; i < 10000; i++ {
		v += i
	}
	return v
}

// benchmarkWithWork runs op b.N times across w goroutines, calling doWork
// between each metric operation to simulate realistic application usage.
// This catches regressions that tight-loop benchmarks miss (see #1748, #1759).
func benchmarkWithWork(b *testing.B, w int, op func()) {
	b.Helper()
	b.StopTimer()

	wg := new(sync.WaitGroup)
	wg.Add(w)

	gate := new(sync.WaitGroup)
	gate.Add(1)

	for i := 0; i < w; i++ {
		go func() {
			gate.Wait()
			for i := 0; i < b.N; i++ {
				runtime.KeepAlive(doWork(i))
				op()
			}
			wg.Done()
		}()
	}

	b.ReportAllocs()
	b.StartTimer()
	gate.Done()
	wg.Wait()
}

func BenchmarkCounterAddWithWork(b *testing.B) {
	c := NewCounter(CounterOpts{
		Name: "benchmark_counter_add",
		Help: "A counter to benchmark float Add (CAS path).",
	})
	for _, w := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprint(w), func(b *testing.B) {
			benchmarkWithWork(b, w, func() { c.Add(1.5) })
		})
	}
}

func BenchmarkCounterIncWithWork(b *testing.B) {
	c := NewCounter(CounterOpts{
		Name: "benchmark_counter_inc",
		Help: "A counter to benchmark Inc (integer fast path).",
	})
	for _, w := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprint(w), func(b *testing.B) {
			benchmarkWithWork(b, w, func() { c.Inc() })
		})
	}
}

func BenchmarkGaugeAddWithWork(b *testing.B) {
	g := NewGauge(GaugeOpts{
		Name: "benchmark_gauge_add",
		Help: "A gauge to benchmark float Add (CAS path).",
	})
	for _, w := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprint(w), func(b *testing.B) {
			benchmarkWithWork(b, w, func() { g.Add(1.5) })
		})
	}
}

func BenchmarkGaugeSetWithWork(b *testing.B) {
	g := NewGauge(GaugeOpts{
		Name: "benchmark_gauge_set",
		Help: "A gauge to benchmark Set (atomic store, no CAS).",
	})
	for _, w := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprint(w), func(b *testing.B) {
			benchmarkWithWork(b, w, func() { g.Set(1.5) })
		})
	}
}

func BenchmarkHistogramObserveWithWork(b *testing.B) {
	h := NewHistogram(HistogramOpts{
		Name: "benchmark_histogram_observe",
		Help: "A histogram to benchmark Observe (partial CAS for sum).",
	})
	for _, w := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprint(w), func(b *testing.B) {
			benchmarkWithWork(b, w, func() { h.Observe(1.5) })
		})
	}
}

func BenchmarkSummaryObserveWithWork(b *testing.B) {
	s := NewSummary(SummaryOpts{
		Name:       "benchmark_summary_observe",
		Help:       "A summary to benchmark Observe (CAS for sum).",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})
	for _, w := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprint(w), func(b *testing.B) {
			benchmarkWithWork(b, w, func() { s.Observe(1.5) })
		})
	}
}
