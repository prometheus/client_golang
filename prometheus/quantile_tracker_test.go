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
	"math/rand"
	"testing"
)

func checkFloat64(t *testing.T, x, v, tol float64) {
	if math.Abs(x-v) > tol {
		t.Errorf("Value %f is not within %f of %f", x, tol, v)
	}
}

func TestQEMAObserve(t *testing.T) {
	// Deliberately start with an estimate outside of tolerance.
	tr := NewQEMATracker(0.5, 0.01, 0.001, 0.1)
	for i := 0; i < 20000; i++ {
		v := rand.NormFloat64()
		tr.Observe(v)
	}
	checkFloat64(t, tr.Estimate(), 0.0, 0.2)
}

func TestQEMAObserve2(t *testing.T) {
	// Deliberately start with an estimate outside of tolerance.
	tr := NewQEMATracker(0.5, 0.01, 0.001, 0.0)
	for i := 0; i < 20000; i++ {
		v := rand.NormFloat64() + 3.0
		tr.Observe(v)
	}
	checkFloat64(t, tr.Estimate(), 3.0, 0.11)
}

func BenchmarkQEMAObserve(b *testing.B) {
	tr := NewQEMATracker(0.5, 0.01, 0.001, 0.1)
	v := 1.0 / float64(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Observe(v)
	}
}

func TestMQEMAObserve(t *testing.T) {
	tr := NewMQEMATracker([]float64{0.25, 0.5, 0.75, 0.9, 0.95, 0.99}, 0.005, 0.01, 0.01)
	for i := 0; i < 100000; i++ {
		v := rand.NormFloat64()
		tr.Observe(v)
	}
	checkFloat64(t, tr.est[0], -0.674, 0.1)
	checkFloat64(t, tr.est[1], 0.0, 0.1)
	checkFloat64(t, tr.est[2], 0.674, 0.1)
	checkFloat64(t, tr.est[3], 1.28155, 0.1)
	checkFloat64(t, tr.est[4], 1.64485, 0.2)
	checkFloat64(t, tr.est[5], 2.43635, 0.4)
}

func BenchmarkMQEMAObserve(b *testing.B) {
	tr := NewMQEMATracker([]float64{0.25, 0.5, 0.75}, 0.005, 0.01, 0.01)
	v := 1.0 / float64(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Observe(v)
	}
}

func BenchmarkMQEMAObserve2(b *testing.B) {
	tr := NewMQEMATracker([]float64{0.25, 0.5, 0.75, 0.9, 0.95, 0.99}, 0.005, 0.01, 0.01)
	v := 1.0 / float64(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Observe(v)
	}
}

func BenchmarkMQEMAObserve3(b *testing.B) {
	tr := NewMQEMATracker([]float64{0.001, 0.01, 0.05, 0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99, 0.999}, 0.005, 0.01, 0.01)
	v := 1.0 / float64(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Observe(v)
	}
}

func TestMSPIObserve(t *testing.T) {
	// Deliberately start with an estimate outside of tolerance.
	tr := NewMSPITracker(0.5, 0.001, 1e-6, 0.1)
	for i := 0; i < 20000; i++ {
		v := rand.NormFloat64()
		tr.Observe(v)
	}
	checkFloat64(t, tr.Estimate(), 0.0, 0.02)
}

func TestMSPIObserve2(t *testing.T) {
	// Deliberately start with an estimate outside of tolerance.
	tr := NewMSPITracker(0.5, 0.01, 1e-6, 0.0)
	for i := 0; i < 20000; i++ {
		v := rand.NormFloat64() + 3.0
		tr.Observe(v)
	}
	checkFloat64(t, tr.Estimate(), 3.0, 0.1)
}

func BenchmarkMSPIObserve(b *testing.B) {
	tr := NewMSPITracker(0.5, 0.01, 1e-6, 0.1)
	v := 1.0 / float64(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Observe(v)
	}
}

func TestMMSPIObserve(t *testing.T) {
	tr := NewMMSPITracker([]float64{0.25, 0.5, 0.75, 0.9, 0.95, 0.99}, 0.005, 0.01, 1e-6)
	for i := 0; i < 100000; i++ {
		v := rand.NormFloat64()
		tr.Observe(v)
	}
	est := tr.Estimate()
	checkFloat64(t, est[0], -0.674, 0.1)
	checkFloat64(t, est[1], 0.0, 0.1)
	checkFloat64(t, est[2], 0.674, 0.2)
	checkFloat64(t, est[3], 1.28155, 0.2)
	checkFloat64(t, est[4], 1.64485, 0.3)
	checkFloat64(t, est[5], 2.43635, 0.3)
}

func BenchmarkMMSPIObserve(b *testing.B) {
	tr := NewMMSPITracker([]float64{0.25, 0.5, 0.75}, 0.005, 0.01, 1e-6)
	v := 1.0 / float64(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Observe(v)
	}
}

func BenchmarkMMSPIObserve2(b *testing.B) {
	tr := NewMMSPITracker([]float64{0.25, 0.5, 0.75, 0.9, 0.95, 0.99}, 0.005, 0.01, 1e-6)
	v := 1.0 / float64(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Observe(v)
	}
}

func BenchmarkMMSPIObserve3(b *testing.B) {
	tr := NewMMSPITracker([]float64{0.001, 0.01, 0.05, 0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99, 0.999}, 0.005, 0.01, 1e-6)
	v := 1.0 / float64(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Observe(v)
	}
}
