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
	"reflect"
	"strconv"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestDelete(t *testing.T) {
	vec := NewGaugeVec(
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		[]string{"l1", "l2"},
	)
	testDelete(t, vec)
}

func TestDeleteWithCollisions(t *testing.T) {
	vec := NewGaugeVec(
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		[]string{"l1", "l2"},
	)
	vec.hashAdd = func(h uint64, s string) uint64 { return 1 }
	vec.hashAddByte = func(h uint64, b byte) uint64 { return 1 }
	testDelete(t, vec)
}

func TestDeleteWithConstraints(t *testing.T) {
	vec := V2.NewGaugeVec(GaugeVecOpts{
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		ConstrainedLabels{
			{Name: "l1"},
			{Name: "l2", Constraint: func(s string) string { return "x" + s }},
		},
	})
	testDelete(t, vec)
}

func testDelete(t *testing.T, vec *GaugeVec) {
	if got, want := vec.Delete(Labels{"l1": "v1", "l2": "v2"}), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).Set(42)
	if got, want := vec.Delete(Labels{"l1": "v1", "l2": "v2"}), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.Delete(Labels{"l1": "v1", "l2": "v2"}), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).Set(42)
	if got, want := vec.Delete(Labels{"l2": "v2", "l1": "v1"}), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.Delete(Labels{"l2": "v2", "l1": "v1"}), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).Set(42)
	if got, want := vec.Delete(Labels{"l2": "v1", "l1": "v2"}), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.Delete(Labels{"l1": "v1"}), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDeleteLabelValues(t *testing.T) {
	vec := NewGaugeVec(
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		[]string{"l1", "l2"},
	)
	testDeleteLabelValues(t, vec)
}

func TestDeleteLabelValuesWithCollisions(t *testing.T) {
	vec := NewGaugeVec(
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		[]string{"l1", "l2"},
	)
	vec.hashAdd = func(h uint64, s string) uint64 { return 1 }
	vec.hashAddByte = func(h uint64, b byte) uint64 { return 1 }
	testDeleteLabelValues(t, vec)
}

func TestDeleteLabelValuesWithConstraints(t *testing.T) {
	vec := V2.NewGaugeVec(GaugeVecOpts{
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		ConstrainedLabels{
			{Name: "l1"},
			{Name: "l2", Constraint: func(s string) string { return "x" + s }},
		},
	})
	testDeleteLabelValues(t, vec)
}

func testDeleteLabelValues(t *testing.T, vec *GaugeVec) {
	if got, want := vec.DeleteLabelValues("v1", "v2"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).Set(42)
	vec.With(Labels{"l1": "v1", "l2": "v3"}).Set(42) // Add junk data for collision.
	if got, want := vec.DeleteLabelValues("v1", "v2"), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.DeleteLabelValues("v1", "v2"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.DeleteLabelValues("v1", "v3"), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).Set(42)
	// Delete out of order.
	if got, want := vec.DeleteLabelValues("v2", "v1"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.DeleteLabelValues("v1"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDeletePartialMatch(t *testing.T) {
	vec := NewGaugeVec(
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		[]string{"l1", "l2", "l3"},
	)
	testDeletePartialMatch(t, vec)
}

func TestDeletePartialMatchWithConstraints(t *testing.T) {
	vec := V2.NewGaugeVec(GaugeVecOpts{
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		ConstrainedLabels{
			{Name: "l1"},
			{Name: "l2", Constraint: func(s string) string { return "x" + s }},
			{Name: "l3"},
		},
	})
	testDeletePartialMatch(t, vec)
}

func testDeletePartialMatch(t *testing.T, baseVec *GaugeVec) {
	assertNoMetric := func(t *testing.T) {
		if n := len(baseVec.metrics); n != 0 {
			t.Error("expected no metrics, got", n)
		}
	}

	// No metric value is set.
	if got, want := baseVec.DeletePartialMatch(Labels{"l1": "v1", "l2": "v2"}), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	baseVec.With(Labels{"l1": "baseValue1", "l2": "baseValue2", "l3": "baseValue3"}).Inc()
	baseVec.With(Labels{"l1": "multiDeleteV1", "l2": "diff1BaseValue2", "l3": "v3"}).Set(42)
	baseVec.With(Labels{"l1": "multiDeleteV1", "l2": "diff2BaseValue2", "l3": "v3"}).Set(84)
	baseVec.With(Labels{"l1": "multiDeleteV1", "l2": "diff3BaseValue2", "l3": "v3"}).Set(168)

	curriedVec := baseVec.MustCurryWith(Labels{"l2": "curriedValue2"})
	curriedVec.WithLabelValues("curriedValue1", "curriedValue3").Inc()
	curriedVec.WithLabelValues("curriedValue1", "differentCurriedValue3").Inc()
	curriedVec.WithLabelValues("differentCurriedValue1", "differentCurriedValue3").Inc()

	// Try to delete nonexistent label with existent value from curried vector.
	if got, want := curriedVec.DeletePartialMatch(Labels{"lx": "curriedValue1"}), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Try to delete valid label with nonexistent value from curried vector.
	if got, want := curriedVec.DeletePartialMatch(Labels{"l1": "badValue1"}), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Try to delete from a curried vector based on labels which were curried.
	// This operation succeeds when run against the base vector below.
	if got, want := curriedVec.DeletePartialMatch(Labels{"l2": "curriedValue2"}), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Try to delete from a curried vector based on labels which were curried,
	// but the value actually exists in the base vector.
	if got, want := curriedVec.DeletePartialMatch(Labels{"l2": "baseValue2"}), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Delete multiple matching metrics from a curried vector based on partial values.
	if got, want := curriedVec.DeletePartialMatch(Labels{"l1": "curriedValue1"}), 2; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Try to delete nonexistent label with existent value from base vector.
	if got, want := baseVec.DeletePartialMatch(Labels{"lx": "curriedValue1"}), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Try to delete partially invalid labels from base vector.
	if got, want := baseVec.DeletePartialMatch(Labels{"l1": "baseValue1", "l2": "badValue2"}), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Delete from the base vector based on values which were curried.
	// This operation fails when run against a curried vector above.
	if got, want := baseVec.DeletePartialMatch(Labels{"l2": "curriedValue2"}), 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Delete multiple metrics from the base vector based on a single valid label.
	if got, want := baseVec.DeletePartialMatch(Labels{"l1": "multiDeleteV1"}), 3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Delete from the base vector based on values which were not curried.
	if got, want := baseVec.DeletePartialMatch(Labels{"l3": "baseValue3"}), 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// All metrics should have been deleted now.
	assertNoMetric(t)
}

func TestMetricVec(t *testing.T) {
	vec := NewGaugeVec(
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		[]string{"l1", "l2"},
	)
	testMetricVec(t, vec)
}

func TestMetricVecWithCollisions(t *testing.T) {
	vec := NewGaugeVec(
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		[]string{"l1", "l2"},
	)
	vec.hashAdd = func(h uint64, s string) uint64 { return 1 }
	vec.hashAddByte = func(h uint64, b byte) uint64 { return 1 }
	testMetricVec(t, vec)
}

func testMetricVec(t *testing.T, vec *GaugeVec) {
	vec.Reset() // Actually test Reset now!

	var pair [2]string
	// Keep track of metrics.
	expected := map[[2]string]int{}

	for i := 0; i < 1000; i++ {
		pair[0], pair[1] = strconv.Itoa(i%4), strconv.Itoa(i%5) // Varying combinations multiples.
		expected[pair]++
		vec.WithLabelValues(pair[0], pair[1]).Inc()

		expected[[2]string{"v1", "v2"}]++
		vec.WithLabelValues("v1", "v2").Inc()
	}

	var total int
	for _, metrics := range vec.metrics {
		for _, metric := range metrics {
			total++
			copy(pair[:], metric.values)

			var metricOut dto.Metric
			if err := metric.metric.Write(&metricOut); err != nil {
				t.Fatal(err)
			}
			actual := *metricOut.Gauge.Value

			var actualPair [2]string
			for i, label := range metricOut.Label {
				actualPair[i] = *label.Value
			}

			// Test output pair against metric.values to ensure we've selected
			// the right one. We check this to ensure the below check means
			// anything at all.
			if actualPair != pair {
				t.Fatalf("unexpected pair association in metric map: %v != %v", actualPair, pair)
			}

			if actual != float64(expected[pair]) {
				t.Fatalf("incorrect counter value for %v: %v != %v", pair, actual, expected[pair])
			}
		}
	}

	if total != len(expected) {
		t.Fatalf("unexpected number of metrics: %v != %v", total, len(expected))
	}

	vec.Reset()

	if len(vec.metrics) > 0 {
		t.Fatalf("reset failed")
	}
}

func TestMetricVecWithConstraints(t *testing.T) {
	constraint := func(s string) string { return "x" + s }
	vec := V2.NewGaugeVec(GaugeVecOpts{
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		ConstrainedLabels{
			{Name: "l1"},
			{Name: "l2", Constraint: constraint},
		},
	})
	testConstrainedMetricVec(t, vec, constraint)
}

func testConstrainedMetricVec(t *testing.T, vec *GaugeVec, constrain func(string) string) {
	vec.Reset() // Actually test Reset now!

	var pair [2]string
	// Keep track of metrics.
	expected := map[[2]string]int{}

	for i := 0; i < 1000; i++ {
		pair[0], pair[1] = strconv.Itoa(i%4), strconv.Itoa(i%5) // Varying combinations multiples.
		expected[[2]string{pair[0], constrain(pair[1])}]++
		vec.WithLabelValues(pair[0], pair[1]).Inc()

		expected[[2]string{"v1", constrain("v2")}]++
		vec.WithLabelValues("v1", "v2").Inc()
	}

	var total int
	for _, metrics := range vec.metrics {
		for _, metric := range metrics {
			total++
			copy(pair[:], metric.values)

			var metricOut dto.Metric
			if err := metric.metric.Write(&metricOut); err != nil {
				t.Fatal(err)
			}
			actual := *metricOut.Gauge.Value

			var actualPair [2]string
			for i, label := range metricOut.Label {
				actualPair[i] = *label.Value
			}

			// Test output pair against metric.values to ensure we've selected
			// the right one. We check this to ensure the below check means
			// anything at all.
			if actualPair != pair {
				t.Fatalf("unexpected pair association in metric map: %v != %v", actualPair, pair)
			}

			if actual != float64(expected[pair]) {
				t.Fatalf("incorrect counter value for %v: %v != %v", pair, actual, expected[pair])
			}
		}
	}

	if total != len(expected) {
		t.Fatalf("unexpected number of metrics: %v != %v", total, len(expected))
	}

	vec.Reset()

	if len(vec.metrics) > 0 {
		t.Fatalf("reset failed")
	}
}

func TestCounterVecEndToEndWithCollision(t *testing.T) {
	vec := NewCounterVec(
		CounterOpts{
			Name: "test",
			Help: "helpless",
		},
		[]string{"labelname"},
	)
	vec.WithLabelValues("77kepQFQ8Kl").Inc()
	vec.WithLabelValues("!0IC=VloaY").Add(2)

	m := &dto.Metric{}
	if err := vec.WithLabelValues("77kepQFQ8Kl").Write(m); err != nil {
		t.Fatal(err)
	}
	if got, want := m.GetLabel()[0].GetValue(), "77kepQFQ8Kl"; got != want {
		t.Errorf("got label value %q, want %q", got, want)
	}
	if got, want := m.GetCounter().GetValue(), 1.; got != want {
		t.Errorf("got value %f, want %f", got, want)
	}
	m.Reset()
	if err := vec.WithLabelValues("!0IC=VloaY").Write(m); err != nil {
		t.Fatal(err)
	}
	if got, want := m.GetLabel()[0].GetValue(), "!0IC=VloaY"; got != want {
		t.Errorf("got label value %q, want %q", got, want)
	}
	if got, want := m.GetCounter().GetValue(), 2.; got != want {
		t.Errorf("got value %f, want %f", got, want)
	}
}

func TestCurryVec(t *testing.T) {
	vec := NewCounterVec(
		CounterOpts{
			Name: "test",
			Help: "helpless",
		},
		[]string{"one", "two", "three"},
	)
	testCurryVec(t, vec)
}

func TestCurryVecWithCollisions(t *testing.T) {
	vec := NewCounterVec(
		CounterOpts{
			Name: "test",
			Help: "helpless",
		},
		[]string{"one", "two", "three"},
	)
	vec.hashAdd = func(h uint64, s string) uint64 { return 1 }
	vec.hashAddByte = func(h uint64, b byte) uint64 { return 1 }
	testCurryVec(t, vec)
}

func TestCurryVecWithConstraints(t *testing.T) {
	constraint := func(s string) string { return "x" + s }
	t.Run("constrainedLabels overlap variableLabels", func(t *testing.T) {
		vec := V2.NewCounterVec(CounterVecOpts{
			CounterOpts{
				Name: "test",
				Help: "helpless",
			},
			ConstrainedLabels{
				{Name: "one"},
				{Name: "two"},
				{Name: "three", Constraint: constraint},
			},
		})
		testCurryVec(t, vec)
	})
	t.Run("constrainedLabels reducing cardinality", func(t *testing.T) {
		constraint := func(s string) string { return "x" }
		vec := V2.NewCounterVec(CounterVecOpts{
			CounterOpts{
				Name: "test",
				Help: "helpless",
			},
			ConstrainedLabels{
				{Name: "one"},
				{Name: "two"},
				{Name: "three", Constraint: constraint},
			},
		})
		testConstrainedCurryVec(t, vec, constraint)
	})
}

func testCurryVec(t *testing.T, vec *CounterVec) {
	assertMetrics := func(t *testing.T) {
		n := 0
		for _, m := range vec.metrics {
			n += len(m)
		}
		if n != 2 {
			t.Error("expected two metrics, got", n)
		}
		m := &dto.Metric{}
		c1, err := vec.GetMetricWithLabelValues("1", "2", "3")
		if err != nil {
			t.Fatal("unexpected error getting metric:", err)
		}
		c1.Write(m)
		if want, got := 1., m.GetCounter().GetValue(); want != got {
			t.Errorf("want %f as counter value, got %f", want, got)
		}
		m.Reset()
		c2, err := vec.GetMetricWithLabelValues("11", "22", "33")
		if err != nil {
			t.Fatal("unexpected error getting metric:", err)
		}
		c2.Write(m)
		if want, got := 1., m.GetCounter().GetValue(); want != got {
			t.Errorf("want %f as counter value, got %f", want, got)
		}
	}

	assertNoMetric := func(t *testing.T) {
		if n := len(vec.metrics); n != 0 {
			t.Error("expected no metrics, got", n)
		}
	}

	t.Run("zero labels", func(t *testing.T) {
		c1 := vec.MustCurryWith(nil)
		c2 := vec.MustCurryWith(nil)
		c1.WithLabelValues("1", "2", "3").Inc()
		c2.With(Labels{"one": "11", "two": "22", "three": "33"}).Inc()
		assertMetrics(t)
		if !c1.Delete(Labels{"one": "1", "two": "2", "three": "3"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("11", "22", "33") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("first label", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"one": "1"})
		c2 := vec.MustCurryWith(Labels{"one": "11"})
		c1.WithLabelValues("2", "3").Inc()
		c2.With(Labels{"two": "22", "three": "33"}).Inc()
		assertMetrics(t)
		if c1.Delete(Labels{"two": "22", "three": "33"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if c2.DeleteLabelValues("2", "3") {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"two": "2", "three": "3"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("22", "33") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("middle label", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"two": "2"})
		c2 := vec.MustCurryWith(Labels{"two": "22"})
		c1.WithLabelValues("1", "3").Inc()
		c2.With(Labels{"one": "11", "three": "33"}).Inc()
		assertMetrics(t)
		if c1.Delete(Labels{"one": "11", "three": "33"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if c2.DeleteLabelValues("1", "3") {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"one": "1", "three": "3"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("11", "33") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("last label", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"three": "3"})
		c2 := vec.MustCurryWith(Labels{"three": "33"})
		c1.WithLabelValues("1", "2").Inc()
		c2.With(Labels{"one": "11", "two": "22"}).Inc()
		assertMetrics(t)
		if c1.Delete(Labels{"two": "22", "one": "11"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if c2.DeleteLabelValues("1", "2") {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"two": "2", "one": "1"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("11", "22") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("two labels", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"three": "3", "one": "1"})
		c2 := vec.MustCurryWith(Labels{"three": "33", "one": "11"})
		c1.WithLabelValues("2").Inc()
		c2.With(Labels{"two": "22"}).Inc()
		assertMetrics(t)
		if c1.Delete(Labels{"two": "22"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if c2.DeleteLabelValues("2") {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"two": "2"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("22") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("all labels", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"three": "3", "two": "2", "one": "1"})
		c2 := vec.MustCurryWith(Labels{"three": "33", "one": "11", "two": "22"})
		c1.WithLabelValues().Inc()
		c2.With(nil).Inc()
		assertMetrics(t)
		if !c1.Delete(Labels{}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues() {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("double curry", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"three": "3"}).MustCurryWith(Labels{"one": "1"})
		c2 := vec.MustCurryWith(Labels{"three": "33"}).MustCurryWith(Labels{"one": "11"})
		c1.WithLabelValues("2").Inc()
		c2.With(Labels{"two": "22"}).Inc()
		assertMetrics(t)
		if c1.Delete(Labels{"two": "22"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if c2.DeleteLabelValues("2") {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"two": "2"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("22") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("use already curried label", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"three": "3"})
		if _, err := c1.GetMetricWithLabelValues("1", "2", "3"); err == nil {
			t.Error("expected error when using already curried label")
		}
		if _, err := c1.GetMetricWith(Labels{"one": "1", "two": "2", "three": "3"}); err == nil {
			t.Error("expected error when using already curried label")
		}
		assertNoMetric(t)
		c1.WithLabelValues("1", "2").Inc()
		if c1.Delete(Labels{"one": "1", "two": "2", "three": "3"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"one": "1", "two": "2"}) {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("curry already curried label", func(t *testing.T) {
		if _, err := vec.MustCurryWith(Labels{"three": "3"}).CurryWith(Labels{"three": "33"}); err == nil {
			t.Error("currying unexpectedly succeeded")
		} else if err.Error() != `label name "three" is already curried` {
			t.Error("currying returned unexpected error:", err)
		}
	})
	t.Run("unknown label", func(t *testing.T) {
		if _, err := vec.CurryWith(Labels{"foo": "bar"}); err == nil {
			t.Error("currying unexpectedly succeeded")
		} else if err.Error() != "1 unknown label(s) found during currying" {
			t.Error("currying returned unexpected error:", err)
		}
	})
}

func testConstrainedCurryVec(t *testing.T, vec *CounterVec, constraint func(string) string) {
	assertMetrics := func(t *testing.T) {
		n := 0
		for _, m := range vec.metrics {
			n += len(m)
		}
		if n != 2 {
			t.Error("expected two metrics, got", n)
		}
		m := &dto.Metric{}
		c1, err := vec.GetMetricWithLabelValues("1", "2", "3")
		if err != nil {
			t.Fatal("unexpected error getting metric:", err)
		}
		c1.Write(m)
		if want, got := 1., m.GetCounter().GetValue(); want != got {
			t.Errorf("want %f as counter value, got %f", want, got)
		}
		values := map[string]string{}
		for _, label := range m.Label {
			values[*label.Name] = *label.Value
		}
		if want, got := map[string]string{"one": "1", "two": "2", "three": constraint("3")}, values; !reflect.DeepEqual(want, got) {
			t.Errorf("want %v as label values, got %v", want, got)
		}
		m.Reset()
		c2, err := vec.GetMetricWithLabelValues("11", "22", "33")
		if err != nil {
			t.Fatal("unexpected error getting metric:", err)
		}
		c2.Write(m)
		if want, got := 1., m.GetCounter().GetValue(); want != got {
			t.Errorf("want %f as counter value, got %f", want, got)
		}
		values = map[string]string{}
		for _, label := range m.Label {
			values[*label.Name] = *label.Value
		}
		if want, got := map[string]string{"one": "11", "two": "22", "three": constraint("33")}, values; !reflect.DeepEqual(want, got) {
			t.Errorf("want %v as label values, got %v", want, got)
		}
	}

	assertNoMetric := func(t *testing.T) {
		if n := len(vec.metrics); n != 0 {
			t.Error("expected no metrics, got", n)
		}
	}

	t.Run("zero labels", func(t *testing.T) {
		c1 := vec.MustCurryWith(nil)
		c2 := vec.MustCurryWith(nil)
		c1.WithLabelValues("1", "2", "3").Inc()
		c2.With(Labels{"one": "11", "two": "22", "three": "33"}).Inc()
		assertMetrics(t)
		if !c1.Delete(Labels{"one": "1", "two": "2", "three": "3"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("11", "22", "33") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("first label", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"one": "1"})
		c2 := vec.MustCurryWith(Labels{"one": "11"})
		c1.WithLabelValues("2", "3").Inc()
		c2.With(Labels{"two": "22", "three": "33"}).Inc()
		assertMetrics(t)
		if c1.Delete(Labels{"two": "22", "three": "33"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if c2.DeleteLabelValues("2", "3") {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"two": "2", "three": "3"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("22", "33") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("middle label", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"two": "2"})
		c2 := vec.MustCurryWith(Labels{"two": "22"})
		c1.WithLabelValues("1", "3").Inc()
		c2.With(Labels{"one": "11", "three": "33"}).Inc()
		assertMetrics(t)
		if c1.Delete(Labels{"one": "11", "three": "33"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if c2.DeleteLabelValues("1", "3") {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"one": "1", "three": "3"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("11", "33") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("last label (constrained to static value)", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"three": "3"})
		c2 := vec.MustCurryWith(Labels{"three": "33"})
		c1.WithLabelValues("1", "2").Inc()
		c2.With(Labels{"one": "11", "two": "22"}).Inc()
		assertMetrics(t)
		if !c1.Delete(Labels{"two": "22", "one": "11"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("1", "2") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("two labels", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"three": "3", "one": "1"})
		c2 := vec.MustCurryWith(Labels{"three": "33", "one": "11"})
		c1.WithLabelValues("2").Inc()
		c2.With(Labels{"two": "22"}).Inc()
		assertMetrics(t)
		if c1.Delete(Labels{"two": "22"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if c2.DeleteLabelValues("2") {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"two": "2"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("22") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("all labels", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"three": "3", "two": "2", "one": "1"})
		c2 := vec.MustCurryWith(Labels{"three": "33", "one": "11", "two": "22"})
		c1.WithLabelValues().Inc()
		c2.With(nil).Inc()
		assertMetrics(t)
		if !c1.Delete(Labels{}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues() {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("double curry", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"three": "3"}).MustCurryWith(Labels{"one": "1"})
		c2 := vec.MustCurryWith(Labels{"three": "33"}).MustCurryWith(Labels{"one": "11"})
		c1.WithLabelValues("2").Inc()
		c2.With(Labels{"two": "22"}).Inc()
		assertMetrics(t)
		if c1.Delete(Labels{"two": "22"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if c2.DeleteLabelValues("2") {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"two": "2"}) {
			t.Error("deletion failed")
		}
		if !c2.DeleteLabelValues("22") {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("use already curried label", func(t *testing.T) {
		c1 := vec.MustCurryWith(Labels{"three": "3"})
		if _, err := c1.GetMetricWithLabelValues("1", "2", "3"); err == nil {
			t.Error("expected error when using already curried label")
		}
		if _, err := c1.GetMetricWith(Labels{"one": "1", "two": "2", "three": "3"}); err == nil {
			t.Error("expected error when using already curried label")
		}
		assertNoMetric(t)
		c1.WithLabelValues("1", "2").Inc()
		if c1.Delete(Labels{"one": "1", "two": "2", "three": "3"}) {
			t.Error("deletion unexpectedly succeeded")
		}
		if !c1.Delete(Labels{"one": "1", "two": "2"}) {
			t.Error("deletion failed")
		}
		assertNoMetric(t)
	})
	t.Run("curry already curried label", func(t *testing.T) {
		if _, err := vec.MustCurryWith(Labels{"three": "3"}).CurryWith(Labels{"three": "33"}); err == nil {
			t.Error("currying unexpectedly succeeded")
		} else if err.Error() != `label name "three" is already curried` {
			t.Error("currying returned unexpected error:", err)
		}
	})
	t.Run("unknown label", func(t *testing.T) {
		if _, err := vec.CurryWith(Labels{"foo": "bar"}); err == nil {
			t.Error("currying unexpectedly succeeded")
		} else if err.Error() != "1 unknown label(s) found during currying" {
			t.Error("currying returned unexpected error:", err)
		}
	})
}

func BenchmarkMetricVecWithBasic(b *testing.B) {
	benchmarkMetricVecWith(b, Labels{
		"l1": "onevalue",
		"l2": "twovalue",
	})
}

func BenchmarkMetricVecWithLabelValuesBasic(b *testing.B) {
	benchmarkMetricVecWithLabelValues(b, map[string][]string{
		"l1": {"onevalue"},
		"l2": {"twovalue"},
	})
}

func BenchmarkMetricVecWithLabelValues2Keys10ValueCardinality(b *testing.B) {
	benchmarkMetricVecWithLabelValuesCardinality(b, 2, 10)
}

func BenchmarkMetricVecWithLabelValues4Keys10ValueCardinality(b *testing.B) {
	benchmarkMetricVecWithLabelValuesCardinality(b, 4, 10)
}

func BenchmarkMetricVecWithLabelValues2Keys100ValueCardinality(b *testing.B) {
	benchmarkMetricVecWithLabelValuesCardinality(b, 2, 100)
}

func BenchmarkMetricVecWithLabelValues10Keys100ValueCardinality(b *testing.B) {
	benchmarkMetricVecWithLabelValuesCardinality(b, 10, 100)
}

func BenchmarkMetricVecWithLabelValues10Keys1000ValueCardinality(b *testing.B) {
	benchmarkMetricVecWithLabelValuesCardinality(b, 10, 1000)
}

func benchmarkMetricVecWithLabelValuesCardinality(b *testing.B, nkeys, nvalues int) {
	labels := map[string][]string{}

	for i := 0; i < nkeys; i++ {
		var (
			k  = fmt.Sprintf("key-%v", i)
			vs = make([]string, 0, nvalues)
		)
		for j := 0; j < nvalues; j++ {
			vs = append(vs, fmt.Sprintf("value-%v", j))
		}
		labels[k] = vs
	}

	benchmarkMetricVecWithLabelValues(b, labels)
}

func benchmarkMetricVecWith(b *testing.B, labels map[string]string) {
	var keys []string
	for k := range labels {
		keys = append(keys, k)
	}

	vec := NewGaugeVec(
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		keys,
	)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec.With(labels)
	}
}

func benchmarkMetricVecWithLabelValues(b *testing.B, labels map[string][]string) {
	var keys []string
	for k := range labels { // Map order dependent, who cares though.
		keys = append(keys, k)
	}

	values := make([]string, len(labels)) // Value cache for permutations.
	vec := NewGaugeVec(
		GaugeOpts{
			Name: "test",
			Help: "helpless",
		},
		keys,
	)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Varies input across provide map entries based on key size.
		for j, k := range keys {
			candidates := labels[k]
			values[j] = candidates[i%len(candidates)]
		}

		vec.WithLabelValues(values...)
	}
}
