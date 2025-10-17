// Copyright 2025 The Prometheus Authors
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

package version

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/prometheus/client_golang/prometheus"
)

var defaultLabels = []string{"branch", "goarch", "goos", "goversion", "revision", "tags", "version"}

func TestGoVersionCollector(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(NewCollector(
		"foo"),
	)
	result, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := json.Marshal(result); err != nil {
		t.Errorf("json marshalling should not fail, %v", err)
	}

	got := []string{}

	for _, r := range result {
		got = append(got, r.GetName())
		fmt.Println("foo")
		m := r.GetMetric()

		if len(m) != 1 {
			t.Errorf("expected 1 metric, but got %d", len(m))
		}

		lk := []string{}
		for _, lp := range m[0].GetLabel() {
			lk = append(lk, lp.GetName())
		}

		if diff := cmp.Diff(lk, defaultLabels); diff != "" {
			t.Errorf("missmatch (-want +got):\n%s", diff)
		}

	}

	if diff := cmp.Diff(got, []string{"foo_build_info"}); diff != "" {
		t.Errorf("missmatch (-want +got):\n%s", diff)
	}
}

func TestGoVersionCollectorWithLabels(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()

	labels := prometheus.Labels{
		"z-mylabel": "myvalue",
	}
	reg.MustRegister(NewCollector(
		"foo", WithExtraConstLabels(labels)),
	)
	result, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := json.Marshal(result); err != nil {
		t.Errorf("json marshalling should not fail, %v", err)
	}

	got := []string{}

	for _, r := range result {
		got = append(got, r.GetName())
		fmt.Println("foo")
		m := r.GetMetric()

		if len(m) != 1 {
			t.Errorf("expected 1 metric, but got %d", len(m))
		}

		lk := []string{}
		for _, lp := range m[0].GetLabel() {
			lk = append(lk, lp.GetName())
		}

		labels := append(defaultLabels, "z-mylabel")

		if diff := cmp.Diff(lk, labels); diff != "" {
			t.Errorf("missmatch (-want +got):\n%s", diff)
		}

	}

	if diff := cmp.Diff(got, []string{"foo_build_info"}); diff != "" {
		t.Errorf("missmatch (-want +got):\n%s", diff)
	}
}
