// Copyright The Prometheus Authors
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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestNativeHistogramMetrics(t *testing.T) {
	metrics := newHistogramMetrics()
	reg := setupRegistry(metrics)

	latencies := []float64{0.005, 0.05, 0.12, 0.25, 0.85, 2.4}
	sizes := []float64{0, 0.5, 1.0, 64, 512, 4096}
	for i := range latencies {
		metrics.observe(latencies[i], sizes[i])
	}

	gathered, err := reg.Gather()
	if err != nil {
		t.Fatalf("unexpected error gathering metrics: %v", err)
	}

	foundDuration := false
	foundResponseSize := false
	foundDualMode := false

	for _, mf := range gathered {
		switch mf.GetName() {
		case "http_request_duration_seconds":
			foundDuration = true
			if mf.GetType() != dto.MetricType_HISTOGRAM {
				t.Errorf("expected HISTOGRAM type, got %v", mf.GetType())
			}
			h := mf.GetMetric()[0].GetHistogram()
			if h.GetSampleCount() != uint64(len(latencies)) {
				t.Errorf("expected sample count %d, got %d", len(latencies), h.GetSampleCount())
			}
			if h.Schema == nil {
				t.Errorf("expected non-nil schema for native histogram")
			}
		case "http_response_size_bytes":
			foundResponseSize = true
			h := mf.GetMetric()[0].GetHistogram()
			if h.GetSampleCount() != uint64(len(sizes)) {
				t.Errorf("expected sample count %d, got %d", len(sizes), h.GetSampleCount())
			}
			if h.ZeroThreshold == nil || *h.ZeroThreshold != 1 {
				t.Errorf("expected ZeroThreshold 1, got %v", h.ZeroThreshold)
			}
		case "http_request_duration_dualmode_seconds":
			foundDualMode = true
			h := mf.GetMetric()[0].GetHistogram()
			if len(h.GetBucket()) == 0 {
				t.Errorf("expected classic buckets to be populated in dual mode")
			}
			if h.Schema == nil {
				t.Errorf("expected native histogram schema in dual mode")
			}
		}
	}

	if !foundDuration {
		t.Errorf("metric http_request_duration_seconds not gathered")
	}
	if !foundResponseSize {
		t.Errorf("metric http_response_size_bytes not gathered")
	}
	if !foundDualMode {
		t.Errorf("metric http_request_duration_dualmode_seconds not gathered")
	}
}

func TestZeroBucketBoundary(t *testing.T) {
	metrics := newHistogramMetrics()
	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics.responseSize)

	// NativeHistogramZeroThreshold is 1, so observations with |v| <= 1 go to the zero bucket.
	for _, v := range []float64{-1.0, -0.5, 0.0, 0.5, 1.0} {
		metrics.responseSize.Observe(v)
	}
	// Observations with |v| > 1 go to sparse exponential buckets.
	metrics.responseSize.Observe(2.0)

	gathered, err := reg.Gather()
	if err != nil {
		t.Fatalf("unexpected error gathering metrics: %v", err)
	}
	if len(gathered) != 1 {
		t.Fatalf("expected 1 metric family, got %d", len(gathered))
	}

	h := gathered[0].GetMetric()[0].GetHistogram()
	if h.GetZeroCount() != 5 {
		t.Errorf("expected zero bucket count 5 for |v| <= 1, got %d", h.GetZeroCount())
	}
	if h.GetSampleCount() != 6 {
		t.Errorf("expected total sample count 6, got %d", h.GetSampleCount())
	}
}

func TestBucketLimitReset(t *testing.T) {
	reg := prometheus.NewRegistry()
	h := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                            "test_reset_histogram",
		Help:                            "Histogram for testing bucket-limit reset behavior.",
		Buckets:                         []float64{1, 10},
		NativeHistogramBucketFactor:     1.1,
		NativeHistogramMaxBucketNumber:  2,
		NativeHistogramMinResetDuration: time.Millisecond,
	})
	reg.MustRegister(h)

	// Populate more than NativeHistogramMaxBucketNumber buckets.
	for _, v := range []float64{1, 2, 4, 8} {
		h.Observe(v)
	}

	// Wait past NativeHistogramMinResetDuration, then observe another value to trigger reset.
	time.Sleep(5 * time.Millisecond)
	h.Observe(16)

	gathered, err := reg.Gather()
	if err != nil {
		t.Fatalf("unexpected error gathering metrics: %v", err)
	}
	hist := gathered[0].GetMetric()[0].GetHistogram()
	if hist.GetSampleCount() != 1 {
		t.Errorf("expected sample count 1 after reset, got %d", hist.GetSampleCount())
	}
	if hist.GetSampleSum() != 16 {
		t.Errorf("expected sample sum 16 after reset, got %v", hist.GetSampleSum())
	}
	if len(hist.GetBucket()) == 0 || hist.GetBucket()[0].GetCumulativeCount() != 0 {
		t.Errorf("expected classic buckets to be reset as well, got %v", hist.GetBucket())
	}
}

func TestDualModeNegotiation(t *testing.T) {
	metrics := newHistogramMetrics()
	reg := setupRegistry(metrics)
	metrics.observe(0.15, 256)

	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})

	// Text format should expose classic buckets for the dual-mode histogram.
	textReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	textReq.Header.Set("Accept", "text/plain;version=0.0.4")
	textRec := httptest.NewRecorder()
	handler.ServeHTTP(textRec, textReq)
	if textRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for text scrape, got %d", textRec.Code)
	}
	if !strings.Contains(textRec.Body.String(), "http_request_duration_dualmode_seconds_bucket{") {
		t.Errorf("expected classic buckets in text response, got:\n%s", textRec.Body.String())
	}

	// Protobuf format should negotiate application/vnd.google.protobuf.
	protoReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	protoReq.Header.Set("Accept", "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited")
	protoRec := httptest.NewRecorder()
	handler.ServeHTTP(protoRec, protoReq)
	if protoRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for protobuf scrape, got %d", protoRec.Code)
	}
	if ct := protoRec.Header().Get("Content-Type"); !strings.Contains(ct, "application/vnd.google.protobuf") {
		t.Errorf("expected protobuf content-type, got %q", ct)
	}
}
