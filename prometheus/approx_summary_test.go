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
	"math"
	"math/rand"
	"sort"
	"sync"
	"testing"
	"testing/quick"

	dto "github.com/prometheus/client_model/go"
)

func benchmarkApproxSummaryObserve(w int, b *testing.B) {
	b.StopTimer()

	wg := new(sync.WaitGroup)
	wg.Add(w)

	g := new(sync.WaitGroup)
	g.Add(1)

	objMap := map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
	s := NewApproxSummary(ApproxSummaryOpts{
		Name:       "test_summary",
		Help:       "helpless",
		Objectives: objMap,
	})

	for i := 0; i < w; i++ {
		go func() {
			g.Wait()

			for i := 0; i < b.N; i++ {
				s.Observe(float64(i))
			}

			wg.Done()
		}()
	}

	b.StartTimer()
	g.Done()
	wg.Wait()
}

func BenchmarkApproxSummaryObserve1(b *testing.B) {
	benchmarkApproxSummaryObserve(1, b)
}

func BenchmarkApproxSummaryObserve2(b *testing.B) {
	benchmarkApproxSummaryObserve(2, b)
}

func BenchmarkApproxSummaryObserve4(b *testing.B) {
	benchmarkApproxSummaryObserve(4, b)
}

func BenchmarkApproxSummaryObserve8(b *testing.B) {
	benchmarkApproxSummaryObserve(8, b)
}

func benchmarkApproxSummaryWrite(w int, b *testing.B) {
	b.StopTimer()

	wg := new(sync.WaitGroup)
	wg.Add(w)

	g := new(sync.WaitGroup)
	g.Add(1)

	objMap := map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
	s := NewApproxSummary(ApproxSummaryOpts{
		Name:       "test_summary",
		Help:       "helpless",
		Objectives: objMap,
	})

	for i := 0; i < 1000000; i++ {
		s.Observe(float64(i))
	}

	for j := 0; j < w; j++ {
		outs := make([]dto.Metric, b.N)

		go func(o []dto.Metric) {
			g.Wait()

			for i := 0; i < b.N; i++ {
				s.Write(&o[i])
			}

			wg.Done()
		}(outs)
	}

	b.StartTimer()
	g.Done()
	wg.Wait()
}

func BenchmarkApproxSummaryWrite1(b *testing.B) {
	benchmarkApproxSummaryWrite(1, b)
}

func BenchmarkApproxSummaryWrite2(b *testing.B) {
	benchmarkApproxSummaryWrite(2, b)
}

func BenchmarkApproxSummaryWrite4(b *testing.B) {
	benchmarkApproxSummaryWrite(4, b)
}

func BenchmarkApproxSummaryWrite8(b *testing.B) {
	benchmarkApproxSummaryWrite(8, b)
}

func TestApproxApproxSummaryConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode.")
	}

	rand.Seed(42)
	objMap := map[float64]float64{0.5: 0.05, 0.9: 0.1, 0.99: 0.1}

	it := func(n uint32) bool {
		mutations := int(n%1e4 + 1e4)
		concLevel := int(n%5 + 1)
		total := mutations * concLevel

		var start, end sync.WaitGroup
		start.Add(1)
		end.Add(concLevel)

		sum := NewApproxSummary(ApproxSummaryOpts{
			Name:       "test_summary",
			Help:       "helpless",
			Objectives: objMap,
			Lam:        0.001,
			Gam:        0.001,
			Rho:        1e-5,
		})

		allVars := make([]float64, total)
		var sampleSum float64
		for i := 0; i < concLevel; i++ {
			vals := make([]float64, mutations)
			for j := 0; j < mutations; j++ {
				v := rand.NormFloat64()
				vals[j] = v
				allVars[i*mutations+j] = v
				sampleSum += v
			}

			go func(vals []float64) {
				start.Wait()
				for _, v := range vals {
					sum.Observe(v)
				}
				end.Done()
			}(vals)
		}
		sort.Float64s(allVars)
		start.Done()
		end.Wait()

		m := &dto.Metric{}
		sum.Write(m)
		if got, want := int(*m.Summary.SampleCount), total; got != want {
			t.Errorf("got sample count %d, want %d", got, want)
		}
		if got, want := *m.Summary.SampleSum, sampleSum; math.Abs((got-want)/want) > 0.001 {
			t.Errorf("got sample sum %f, want %f", got, want)
		}

		objSlice := make([]float64, 0, len(objMap))
		for qu := range objMap {
			objSlice = append(objSlice, qu)
		}
		sort.Float64s(objSlice)

		for i, wantQ := range objSlice {
			ε := objMap[wantQ]
			gotQ := *m.Summary.Quantile[i].Quantile
			gotV := *m.Summary.Quantile[i].Value
			min, max := getBounds(allVars, wantQ, ε)
			if gotQ != wantQ {
				t.Errorf("got quantile %f, want %f", gotQ, wantQ)
			}
			if gotV < min || gotV > max {
				t.Errorf("got %f for quantile %f, want [%f,%f]", gotV, gotQ, min, max)
			}
		}
		return true
	}

	if err := quick.Check(it, nil); err != nil {
		t.Error(err)
	}
}

func TestApproxSummaryVecConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode.")
	}

	rand.Seed(42)
	objMap := map[float64]float64{0.5: 0.05, 0.9: 0.1, 0.99: 0.1}

	objSlice := make([]float64, 0, len(objMap))
	for qu := range objMap {
		objSlice = append(objSlice, qu)
	}
	sort.Float64s(objSlice)

	it := func(n uint32) bool {
		mutations := int(n%1e4 + 1e4)
		concLevel := int(n%7 + 1)
		vecLength := int(n%3 + 1)

		var start, end sync.WaitGroup
		start.Add(1)
		end.Add(concLevel)

		sum := NewApproxSummaryVec(
			ApproxSummaryOpts{
				Name:       "test_summary",
				Help:       "helpless",
				Objectives: objMap,
				Lam:        0.001,
				Gam:        0.001,
				Rho:        1e-5,
			},
			[]string{"label"},
		)

		allVars := make([][]float64, vecLength)
		sampleSums := make([]float64, vecLength)
		for i := 0; i < concLevel; i++ {
			vals := make([]float64, mutations)
			picks := make([]int, mutations)
			for j := 0; j < mutations; j++ {
				v := rand.NormFloat64()
				vals[j] = v
				pick := rand.Intn(vecLength)
				picks[j] = pick
				allVars[pick] = append(allVars[pick], v)
				sampleSums[pick] += v
			}

			go func(vals []float64) {
				start.Wait()
				for i, v := range vals {
					sum.WithLabelValues(fmt.Sprintf("%d", picks[i])).Observe(v)
				}
				end.Done()
			}(vals)
		}
		for _, vars := range allVars {
			sort.Float64s(vars)
		}
		start.Done()
		end.Wait()

		for i := 0; i < vecLength; i++ {
			m := &dto.Metric{}
			s := sum.WithLabelValues(fmt.Sprintf("%d", i))
			s.(Summary).Write(m)
			if got, want := int(*m.Summary.SampleCount), len(allVars[i]); got != want {
				t.Errorf("got sample count %d for label %c, want %d", got, i, want)
			}
			if got, want := *m.Summary.SampleSum, sampleSums[i]; math.Abs((got-want)/want) > 0.001 {
				t.Errorf("got sample sum %f for label %c, want %f", got, i, want)
			}
			for j, wantQ := range objSlice {
				ε := objMap[wantQ]
				gotQ := *m.Summary.Quantile[j].Quantile
				gotV := *m.Summary.Quantile[j].Value
				min, max := getBounds(allVars[i], wantQ, ε)
				if gotQ != wantQ {
					t.Errorf("got quantile %f for label %c, want %f", gotQ, i, wantQ)
				}
				if gotV < min || gotV > max {
					t.Errorf("got %f for quantile %f for label %c, want [%f,%f]", gotV, gotQ, i, min, max)
				}
			}
		}
		return true
	}

	if err := quick.Check(it, nil); err != nil {
		t.Error(err)
	}
}
