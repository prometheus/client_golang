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
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestDelete(t *testing.T) {
	vec := newUntypedMetricVec("test", "helpless", []string{"l1", "l2"})
	testDelete(t, vec)
}

func TestDeleteWithCollisions(t *testing.T) {
	vec := newUntypedMetricVec("test", "helpless", []string{"l1", "l2"})
	vec.hashAdd = func(h uint64, s string) uint64 { return 1 }
	vec.hashAddByte = func(h uint64, b byte) uint64 { return 1 }
	testDelete(t, vec)
}

func testDelete(t *testing.T, vec *MetricVec) {
	if got, want := vec.Delete(Labels{"l1": "v1", "l2": "v2"}), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).(Untyped).Set(42)
	if got, want := vec.Delete(Labels{"l1": "v1", "l2": "v2"}), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.Delete(Labels{"l1": "v1", "l2": "v2"}), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).(Untyped).Set(42)
	if got, want := vec.Delete(Labels{"l2": "v2", "l1": "v1"}), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.Delete(Labels{"l2": "v2", "l1": "v1"}), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).(Untyped).Set(42)
	if got, want := vec.Delete(Labels{"l2": "v1", "l1": "v2"}), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.Delete(Labels{"l1": "v1"}), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDeleteLabelValues(t *testing.T) {
	vec := newUntypedMetricVec("test", "helpless", []string{"l1", "l2"})
	testDeleteLabelValues(t, vec)
}

func TestDeleteLabelValuesWithCollisions(t *testing.T) {
	vec := newUntypedMetricVec("test", "helpless", []string{"l1", "l2"})
	vec.hashAdd = func(h uint64, s string) uint64 { return 1 }
	vec.hashAddByte = func(h uint64, b byte) uint64 { return 1 }
	testDeleteLabelValues(t, vec)
}

func testDeleteLabelValues(t *testing.T, vec *MetricVec) {
	if got, want := vec.DeleteLabelValues("v1", "v2"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).(Untyped).Set(42)
	vec.With(Labels{"l1": "v1", "l2": "v3"}).(Untyped).Set(42) // add junk data for collision
	if got, want := vec.DeleteLabelValues("v1", "v2"), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.DeleteLabelValues("v1", "v2"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.DeleteLabelValues("v1", "v3"), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).(Untyped).Set(42)
	// delete out of order
	if got, want := vec.DeleteLabelValues("v2", "v1"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.DeleteLabelValues("v1"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMetricVec(t *testing.T) {
	vec := newUntypedMetricVec("test", "helpless", []string{"l1", "l2"})
	testMetricVec(t, vec)
}

func TestMetricVecWithCollisions(t *testing.T) {
	vec := newUntypedMetricVec("test", "helpless", []string{"l1", "l2"})
	vec.hashAdd = func(h uint64, s string) uint64 { return 1 }
	vec.hashAddByte = func(h uint64, b byte) uint64 { return 1 }
	testMetricVec(t, vec)
}

func testMetricVec(t *testing.T, vec *MetricVec) {
	vec.Reset() // actually test Reset now!

	var pair [2]string
	// keep track of metrics
	expected := map[[2]string]int{}

	for i := 0; i < 1000; i++ {
		pair[0], pair[1] = fmt.Sprint(i%4), fmt.Sprint(i%5) // varying combinations multiples
		expected[pair]++
		vec.WithLabelValues(pair[0], pair[1]).(Untyped).Inc()

		expected[[2]string{"v1", "v2"}]++
		vec.WithLabelValues("v1", "v2").(Untyped).Inc()
	}

	var total int
	for _, metrics := range vec.children {
		for _, metric := range metrics {
			total++
			copy(pair[:], metric.values)

			// is there a better way to access the value of a metric?
			var metricOut dto.Metric
			metric.metric.Write(&metricOut)
			actual := *metricOut.Untyped.Value

			var actualPair [2]string
			for i, label := range metricOut.Label {
				actualPair[i] = *label.Value
			}

			// test output pair against metric.values to ensure we've selected
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

	if len(vec.children) > 0 {
		t.Fatalf("reset failed")
	}
}

func newUntypedMetricVec(name, help string, labels []string) *MetricVec {
	desc := NewDesc("test", "helpless", labels, nil)
	vec := newMetricVec(desc, func(lvs ...string) Metric {
		return newValue(desc, UntypedValue, 0, lvs...)
	})
	return &vec
}

func BenchmarkMetricVecWithLabelValuesBasic(B *testing.B) {
	benchmarkMetricVecWithLabelValues(B, map[string][]string{
		"l1": []string{"onevalue"},
		"l2": []string{"twovalue"},
	})
}

func BenchmarkMetricVecWithLabelValues2Keys10ValueCardinality(B *testing.B) {
	benchmarkMetricVecWithLabelValuesCardinality(B, 2, 10)
}

func BenchmarkMetricVecWithLabelValues4Keys10ValueCardinality(B *testing.B) {
	benchmarkMetricVecWithLabelValuesCardinality(B, 4, 10)
}

func BenchmarkMetricVecWithLabelValues2Keys100ValueCardinality(B *testing.B) {
	benchmarkMetricVecWithLabelValuesCardinality(B, 2, 100)
}

func BenchmarkMetricVecWithLabelValues10Keys100ValueCardinality(B *testing.B) {
	benchmarkMetricVecWithLabelValuesCardinality(B, 10, 100)
}

func BenchmarkMetricVecWithLabelValues10Keys1000ValueCardinality(B *testing.B) {
	benchmarkMetricVecWithLabelValuesCardinality(B, 10, 1000)
}

func benchmarkMetricVecWithLabelValuesCardinality(B *testing.B, nkeys, nvalues int) {
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

	benchmarkMetricVecWithLabelValues(B, labels)
}

func benchmarkMetricVecWithLabelValues(B *testing.B, labels map[string][]string) {
	var keys []string
	for k := range labels { // map order dependent, who cares though
		keys = append(keys, k)
	}

	values := make([]string, len(labels)) // value cache for permutations
	vec := newUntypedMetricVec("test", "helpless", keys)

	B.ReportAllocs()
	B.ResetTimer()
	for i := 0; i < B.N; i++ {
		// varies input across provide map entries based on key size.
		for j, k := range keys {
			candidates := labels[k]
			values[j] = candidates[i%len(candidates)]
		}

		vec.WithLabelValues(values...)
	}
}
