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
	"sync"
	"testing"
)

// TestAtomicAddFloatConcurrent verifies that atomicAddFloat produces correct
// results under high contention.
func TestAtomicAddFloatConcurrent(t *testing.T) {
	const (
		goroutines = 100
		additions  = 1000
		addValue   = 1.5
	)

	var valBits uint64
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < additions; j++ {
				atomicAddFloat(&valBits, addValue)
			}
		}()
	}

	wg.Wait()

	expected := float64(goroutines * additions * addValue)
	got := math.Float64frombits(valBits)

	if got != expected {
		t.Errorf("Expected %f, got %f", expected, got)
	}
}

// BenchmarkAtomicAddFloatContention benchmarks atomicAddFloat under various
// contention levels to verify it performs well in both low and high contention.
func BenchmarkAtomicAddFloatContention(b *testing.B) {
	table := []struct {
		name       string
		goroutines int
	}{
		{"goroutines=1", 1},
		{"goroutines=2", 2},
		{"goroutines=4", 4},
		{"goroutines=8", 8},
		{"goroutines=16", 16},
		{"goroutines=32", 32},
		{"goroutines=64", 64},
		{"goroutines=128", 128},
	}

	for _, t := range table {
		b.Run(t.name, func(b *testing.B) {
			var valBits uint64
			var wg sync.WaitGroup

			b.ResetTimer()
			for i := 0; i < t.goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < b.N/t.goroutines; j++ {
						atomicAddFloat(&valBits, 1.5)
					}
				}()
			}
			wg.Wait()
		})
	}
}

// BenchmarkCounterHighContention benchmarks Counter.Add under high contention.
func BenchmarkCounterHighContention(b *testing.B) {
	table := []struct {
		name       string
		goroutines int
	}{
		{"goroutines=1", 1},
		{"goroutines=2", 2},
		{"goroutines=4", 4},
		{"goroutines=8", 8},
		{"goroutines=16", 16},
		{"goroutines=32", 32},
		{"goroutines=64", 64},
		{"goroutines=128", 128},
	}

	for _, t := range table {
		b.Run(t.name, func(b *testing.B) {
			c := NewCounter(CounterOpts{
				Name: "benchmark_counter",
				Help: "A counter to benchmark it.",
			})
			b.ReportAllocs()
			b.ResetTimer()

			var wg sync.WaitGroup
			for i := 0; i < t.goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < b.N/t.goroutines; j++ {
						c.Add(1.5)
					}
				}()
			}
			wg.Wait()
		})
	}
}

// BenchmarkGaugeHighContention benchmarks Gauge.Add under high contention.
func BenchmarkGaugeHighContention(b *testing.B) {
	table := []struct {
		name       string
		goroutines int
	}{
		{"goroutines=1", 1},
		{"goroutines=2", 2},
		{"goroutines=4", 4},
		{"goroutines=8", 8},
		{"goroutines=16", 16},
		{"goroutines=32", 32},
		{"goroutines=64", 64},
		{"goroutines=128", 128},
	}

	for _, t := range table {
		b.Run(t.name, func(b *testing.B) {
			g := NewGauge(GaugeOpts{
				Name: "benchmark_gauge",
				Help: "A gauge to benchmark it.",
			})
			b.ReportAllocs()
			b.ResetTimer()

			var wg sync.WaitGroup
			for i := 0; i < t.goroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < b.N/t.goroutines; j++ {
						g.Add(1.5)
					}
				}()
			}
			wg.Wait()
		})
	}
}
