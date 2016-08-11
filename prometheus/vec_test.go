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
)

func TestDelete(t *testing.T) {
	vec := newUntypedMetricVec("test", "helpless", []string{"l1", "l2"})

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

	if got, want := vec.DeleteLabelValues("v1", "v2"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).(Untyped).Set(42)
	if got, want := vec.DeleteLabelValues("v1", "v2"), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.DeleteLabelValues("v1", "v2"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	vec.With(Labels{"l1": "v1", "l2": "v2"}).(Untyped).Set(42)
	if got, want := vec.DeleteLabelValues("v2", "v1"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := vec.DeleteLabelValues("v1"), false; got != want {
		t.Errorf("got %v, want %v", got, want)
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
