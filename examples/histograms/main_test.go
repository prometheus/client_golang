// Copyright 2026 The Prometheus Authors
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

package main

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestHistogramMetrics(t *testing.T) {
	metrics := newHistogramMetrics()
	reg := setupRegistry(metrics)

	samples := []float64{0.0, 0.0005, 0.05, 0.12, 0.25, 0.85, 2.4}
	for _, s := range samples {
		metrics.observeSample(s)
	}

	gathered, err := reg.Gather()
	if err != nil {
		t.Fatalf("unexpected error gathering metrics: %v", err)
	}

	foundDefault := false
	foundFineTuned := false
	foundDualMode := false

	for _, mf := range gathered {
		switch mf.GetName() {
		case "http_request_duration_default_seconds":
			foundDefault = true
			if mf.GetType() != dto.MetricType_HISTOGRAM {
				t.Errorf("expected HISTOGRAM type, got %v", mf.GetType())
			}
			metric := mf.GetMetric()[0]
			h := metric.GetHistogram()
			if h.GetSampleCount() != uint64(len(samples)) {
				t.Errorf("expected sample count %d, got %d", len(samples), h.GetSampleCount())
			}
			if h.Schema == nil || *h.Schema == 0 {
				t.Errorf("expected positive schema for native histogram, got %v", h.Schema)
			}
		case "http_request_duration_finetuned_seconds":
			foundFineTuned = true
			metric := mf.GetMetric()[0]
			h := metric.GetHistogram()
			if h.GetSampleCount() != uint64(len(samples)) {
				t.Errorf("expected sample count %d, got %d", len(samples), h.GetSampleCount())
			}
			if h.ZeroThreshold == nil || *h.ZeroThreshold != 0.001 {
				t.Errorf("expected ZeroThreshold 0.001, got %v", h.ZeroThreshold)
			}
		case "http_request_duration_dualmode_seconds":
			foundDualMode = true
			metric := mf.GetMetric()[0]
			h := metric.GetHistogram()
			if len(h.GetBucket()) == 0 {
				t.Errorf("expected classic buckets to be populated in dual mode")
			}
			if h.Schema == nil || *h.Schema == 0 {
				t.Errorf("expected native histogram schema in dual mode")
			}
		}
	}

	if !foundDefault {
		t.Errorf("metric http_request_duration_default_seconds not gathered")
	}
	if !foundFineTuned {
		t.Errorf("metric http_request_duration_finetuned_seconds not gathered")
	}
	if !foundDualMode {
		t.Errorf("metric http_request_duration_dualmode_seconds not gathered")
	}
}
