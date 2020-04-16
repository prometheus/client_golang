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
	"strings"
	"sync"
	"testing"
	"testing/quick"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func listenGaugeStream(vals, result chan float64, done chan struct{}) {
	var sum float64
outer:
	for {
		select {
		case <-done:
			close(vals)
			for v := range vals {
				sum += v
			}
			break outer
		case v := <-vals:
			sum += v
		}
	}
	result <- sum
	close(result)
}

func TestGaugeConcurrency(t *testing.T) {
	it := func(n uint32) bool {
		mutations := int(n % 10000)
		concLevel := int(n%15 + 1)

		var start, end sync.WaitGroup
		start.Add(1)
		end.Add(concLevel)

		sStream := make(chan float64, mutations*concLevel)
		result := make(chan float64)
		done := make(chan struct{})

		go listenGaugeStream(sStream, result, done)
		go func() {
			end.Wait()
			close(done)
		}()

		gge := NewGauge(GaugeOpts{
			Name: "test_gauge",
			Help: "no help can be found here",
		})
		for i := 0; i < concLevel; i++ {
			vals := make([]float64, mutations)
			for j := 0; j < mutations; j++ {
				vals[j] = rand.Float64() - 0.5
			}

			go func(vals []float64) {
				start.Wait()
				for _, v := range vals {
					sStream <- v
					gge.Add(v)
				}
				end.Done()
			}(vals)
		}
		start.Done()

		if expected, got := <-result, math.Float64frombits(gge.(*gauge).valBits); math.Abs(expected-got) > 0.000001 {
			t.Fatalf("expected approx. %f, got %f", expected, got)
			return false
		}
		return true
	}

	if err := quick.Check(it, nil); err != nil {
		t.Fatal(err)
	}
}

func TestGaugeVecConcurrency(t *testing.T) {
	it := func(n uint32) bool {
		mutations := int(n % 10000)
		concLevel := int(n%15 + 1)
		vecLength := int(n%5 + 1)

		var start, end sync.WaitGroup
		start.Add(1)
		end.Add(concLevel)

		sStreams := make([]chan float64, vecLength)
		results := make([]chan float64, vecLength)
		done := make(chan struct{})

		for i := 0; i < vecLength; i++ {
			sStreams[i] = make(chan float64, mutations*concLevel)
			results[i] = make(chan float64)
			go listenGaugeStream(sStreams[i], results[i], done)
		}

		go func() {
			end.Wait()
			close(done)
		}()

		gge := NewGaugeVec(
			GaugeOpts{
				Name: "test_gauge",
				Help: "no help can be found here",
			},
			[]string{"label"},
		)
		for i := 0; i < concLevel; i++ {
			vals := make([]float64, mutations)
			pick := make([]int, mutations)
			for j := 0; j < mutations; j++ {
				vals[j] = rand.Float64() - 0.5
				pick[j] = rand.Intn(vecLength)
			}

			go func(vals []float64) {
				start.Wait()
				for i, v := range vals {
					sStreams[pick[i]] <- v
					gge.WithLabelValues(string('A' + pick[i])).Add(v)
				}
				end.Done()
			}(vals)
		}
		start.Done()

		for i := range sStreams {
			if expected, got := <-results[i], math.Float64frombits(gge.WithLabelValues(string('A'+i)).(*gauge).valBits); math.Abs(expected-got) > 0.000001 {
				t.Fatalf("expected approx. %f, got %f", expected, got)
				return false
			}
		}
		return true
	}

	if err := quick.Check(it, nil); err != nil {
		t.Fatal(err)
	}
}

func TestGaugeFunc(t *testing.T) {
	gf := NewGaugeFunc(
		GaugeOpts{
			Name:        "test_name",
			Help:        "test help",
			ConstLabels: Labels{"a": "1", "b": "2"},
		},
		func() float64 { return 3.1415 },
	)

	if expected, got := `Desc{fqName: "test_name", help: "test help", constLabels: {a="1",b="2"}, variableLabels: []}`, gf.Desc().String(); expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}

	m := &dto.Metric{}
	gf.Write(m)

	if expected, got := `label:<name:"a" value:"1" > label:<name:"b" value:"2" > gauge:<value:3.1415 > `, m.String(); expected != got {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestGaugeSetCurrentTime(t *testing.T) {
	g := NewGauge(GaugeOpts{
		Name: "test_name",
		Help: "test help",
	})
	g.SetToCurrentTime()
	unixTime := float64(time.Now().Unix())

	m := &dto.Metric{}
	g.Write(m)

	delta := unixTime - m.GetGauge().GetValue()
	// This is just a smoke test to make sure SetToCurrentTime is not
	// totally off. Tests with current time involved are hard...
	if math.Abs(delta) > 5 {
		t.Errorf("Gauge set to current time deviates from current time by more than 5s, delta is %f seconds", delta)
	}
}

func TestGaugeFuncVecEndToEnd(t *testing.T) {
	gfv := NewGaugeFuncVec(
		GaugeOpts{
			Name:        "test_name",
			Help:        "test help",
			ConstLabels: Labels{"const": "42"},
		},
		[]string{"var", "curried"},
	)
	// add by labels
	err := gfv.Add(func() float64 { return 10 }, Labels{"var": "labels", "curried": "false"})
	if err != nil {
		t.Fatalf("metric should be added correctly: %s", err)
	}

	// add by labels and remove later
	err = gfv.Add(func() float64 { return 20 }, Labels{"var": "to_be_removed", "curried": "false"})
	if err != nil {
		t.Fatalf("metric should be added correctly: %s", err)
	}

	// add by labels and remove + add again later
	err = gfv.Add(func() float64 { return 30 }, Labels{"var": "to_be_replaced", "curried": "false"})
	if err != nil {
		t.Fatalf("metric should be added correctly: %s", err)
	}

	// add by label values
	err = gfv.AddWithLabels(func() float64 { return 40 }, "label_values", "false")
	if err != nil {
		t.Fatalf("metric should be added correctly: %s", err)
	}

	// add by label values and remove later
	err = gfv.AddWithLabels(func() float64 { return 50 }, "label_values_to_be_removed", "false")
	if err != nil {
		t.Fatalf("metric should be added correctly: %s", err)
	}

	// remove the one added by labels by label values
	deleted := gfv.DeleteLabelValues("to_be_removed", "false")
	if !deleted {
		t.Errorf("should be deleted")
	}

	// remove the one added by label values by labels
	deleted = gfv.Delete(Labels{"var": "label_values_to_be_removed", "curried": "false"})
	if !deleted {
		t.Errorf("should be deleted")
	}

	// remove and add again with a new value
	deleted = gfv.DeleteLabelValues("to_be_replaced", "false")
	if !deleted {
		t.Errorf("should be deleted")
	}
	err = gfv.Add(func() float64 { return 60 }, Labels{"var": "to_be_replaced", "curried": "false"})
	if err != nil {
		t.Fatalf("metric should be added correctly: %s", err)
	}

	curriedGfv, err := gfv.CurryWith(Labels{"curried": "true"})
	if err != nil {
		t.Fatalf("should be able to curry: %s", err)
	}

	// add curried in both ways
	err = curriedGfv.AddWithLabels(func() float64 { return 70 }, "label_values")
	if err != nil {
		t.Fatalf("metric should be added correctly: %s", err)
	}
	err = curriedGfv.Add(func() float64 { return 80 }, Labels{"var": "labels"})
	if err != nil {
		t.Fatalf("metric should be added correctly: %s", err)
	}

	metricChan := make(chan Metric)
	go func() {
		gfv.Collect(metricChan)
		close(metricChan)
	}()

	expected := map[string]bool{
		`label:<name:"const" value:"42" > label:<name:"curried" value:"false" > label:<name:"var" value:"labels" > gauge:<value:10 >`:         true,
		`label:<name:"const" value:"42" > label:<name:"curried" value:"false" > label:<name:"var" value:"label_values" > gauge:<value:40 >`:   true,
		`label:<name:"const" value:"42" > label:<name:"curried" value:"false" > label:<name:"var" value:"to_be_replaced" > gauge:<value:60 >`: true,
		`label:<name:"const" value:"42" > label:<name:"curried" value:"true" > label:<name:"var" value:"label_values" > gauge:<value:70 >`:    true,
		`label:<name:"const" value:"42" > label:<name:"curried" value:"true" > label:<name:"var" value:"labels" > gauge:<value:80 >`:          true,
	}

	got := map[string]bool{}
	for metric := range metricChan {
		m := &dto.Metric{}
		if err := metric.Write(m); err != nil {
			t.Fatalf("can't read metric: %s", err)
		}
		got[strings.TrimSpace(m.String())] = true
	}

	for m := range expected {
		if !got[m] {
			t.Errorf("Expected `%s` but didn't get", m)
		}
	}
	for m := range got {
		if !expected[m] {
			t.Errorf("Got unexpected `%s`", m)
		}
	}
}

func TestGaugeFuncVecFailingAdd(t *testing.T) {
	gfv := NewGaugeFuncVec(
		GaugeOpts{
			Name:        "test_name",
			Help:        "test help",
			ConstLabels: Labels{"const": "42"},
		},
		[]string{"var", "curried"},
	)
	f := func() float64 { return 0 }

	expectErr := func(err error) {
		t.Helper()
		if err == nil {
			t.Errorf("should receive an error, got nil")
		}
	}

	expectPanic := func(panicker func()) {
		t.Helper()
		panicked := false
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					panicked = true
				}
			}()
			panicker()
		}()
		if !panicked {
			t.Errorf("should panic, but didn't")
		}
	}

	t.Run("missing label", func(t *testing.T) {
		expectErr(gfv.Add(f, Labels{"var": "missing_curried"}))
		expectPanic(func() {
			gfv.MustAdd(f, Labels{"var": "missing_curried"})
		})
	})

	t.Run("too few labels", func(t *testing.T) {
		expectErr(gfv.AddWithLabels(f, "few"))
		expectPanic(func() {
			gfv.MustAddWithLabels(f, "few")
		})
	})

	t.Run("too many labels", func(t *testing.T) {
		expectErr(gfv.AddWithLabels(f, "too", "many", "labels"))
		expectPanic(func() {
			gfv.MustAddWithLabels(f, "too", "many", "labels")
		})
	})

	t.Run("unexpected label", func(t *testing.T) {
		expectErr(gfv.Add(f, Labels{"var": "ok", "curried": "false", "extra": "unexpected"}))
		expectPanic(func() {
			gfv.MustAdd(f, Labels{"var": "ok", "curried": "false", "unexpected": "label"})
		})
	})

	t.Run("curry unexpected label", func(t *testing.T) {
		expectPanic(func() {
			gfv.MustCurryWith(Labels{"unexpected": "label"})
		})
	})

	t.Run("curry same label", func(t *testing.T) {
		curried := gfv.MustCurryWith(Labels{"curried": "true"})
		expectPanic(func() {
			curried.MustCurryWith(Labels{"curried": "true"})
		})
	})

	t.Run("add curried label", func(t *testing.T) {
		curried := gfv.MustCurryWith(Labels{"curried": "true"})
		expectErr(curried.Add(f, Labels{"var": "ok", "curried": "true"}))
		expectPanic(func() {
			curried.MustAdd(f, Labels{"var": "ok", "curried": "true"})
		})
	})
}
