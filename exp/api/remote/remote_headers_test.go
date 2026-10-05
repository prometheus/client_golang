// Copyright 2024 The Prometheus Authors
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

package remote

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestWriteResponse(t *testing.T) {
	t.Run("new response has empty headers", func(t *testing.T) {
		resp := NewWriteResponse()
		if len(resp.extraHeaders) != 0 {
			t.Errorf("expected empty headers, got %v", resp.extraHeaders)
		}
	})

	t.Run("setters", func(t *testing.T) {
		resp := NewWriteResponse()

		resp.SetStatusCode(http.StatusOK)
		if got := resp.statusCode; got != http.StatusOK {
			t.Errorf("expected status code %d, got %d", http.StatusOK, got)
		}

		stats := WriteResponseStats{
			Samples:    10,
			Histograms: 5,
			Exemplars:  2,
			confirmed:  true,
		}
		resp.Add(stats)
		if diff := cmp.Diff(stats, resp.Stats(), cmpopts.IgnoreUnexported(WriteResponseStats{})); diff != "" {
			t.Errorf("stats mismatch (-want +got):\n%s", diff)
		}

		toAdd := WriteResponseStats{
			Samples:    10,
			Histograms: 5,
			Exemplars:  2,
			confirmed:  true,
		}
		resp.Add(toAdd)
		if diff := cmp.Diff(WriteResponseStats{
			Samples:    20,
			Histograms: 10,
			Exemplars:  4,
			confirmed:  true,
		}, resp.Stats(), cmpopts.IgnoreUnexported(WriteResponseStats{})); diff != "" {
			t.Errorf("stats mismatch (-want +got):\n%s", diff)
		}

		resp.SetExtraHeader("Test-Header", "test-value")
		if got := resp.extraHeaders.Get("Test-Header"); got != "test-value" {
			t.Errorf("expected header value %q, got %q", "test-value", got)
		}
	})

	t.Run("NewWriteResponseWithStats", func(t *testing.T) {
		stats := NewConfirmedWriteResponseStats(10, 5, 2)
		resp := NewWriteResponseWithStats(stats)
		if resp.statusCode != http.StatusNoContent {
			t.Errorf("expected status code %d, got %d", http.StatusNoContent, resp.statusCode)
		}
		if len(resp.extraHeaders) != 0 {
			t.Errorf("expected empty extra headers, got %v", resp.extraHeaders)
		}
		if diff := cmp.Diff(stats, resp.Stats(), cmpopts.IgnoreUnexported(WriteResponseStats{})); diff != "" {
			t.Errorf("stats mismatch (-want +got):\n%s", diff)
		}
		if !resp.Confirmed() {
			t.Error("expected confirmed to be true")
		}
	})

	t.Run("SetStats and SetConfirmed", func(t *testing.T) {
		resp := NewWriteResponse()
		if resp.Confirmed() {
			t.Error("expected newly created response to be unconfirmed")
		}

		stats := NewWriteResponseStats(3, 4, 5, false)
		resp.SetStats(stats)
		if resp.Confirmed() {
			t.Error("expected response to remain unconfirmed")
		}
		if diff := cmp.Diff(stats, resp.Stats(), cmpopts.IgnoreUnexported(WriteResponseStats{})); diff != "" {
			t.Errorf("stats mismatch (-want +got):\n%s", diff)
		}

		resp.SetConfirmed(true)
		if !resp.Confirmed() {
			t.Error("expected response to be confirmed after SetConfirmed(true)")
		}
	})

	t.Run("writeHeaders v1", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV1MessageType, w)

		expectedHeaders := map[string]string{
			"Custom-Header": "custom-value",
		}

		for k, want := range expectedHeaders {
			if got := w.Header().Get(k); got != want {
				t.Errorf("header %q: want %q, got %q", k, want, got)
			}
		}
	})

	t.Run("writeHeaders v1 with confirmed stats", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.Add(NewConfirmedWriteResponseStats(10, 5, 2))
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV1MessageType, w)

		if got := w.Header().Get("Custom-Header"); got != "custom-value" {
			t.Errorf("expected Custom-Header to be custom-value, got %q", got)
		}
		if got := w.Header().Get(writtenSamplesHeader); got != "" {
			t.Errorf("expected no samples header for v1, got %q", got)
		}
		if got := w.Header().Get(writtenHistogramsHeader); got != "" {
			t.Errorf("expected no histograms header for v1, got %q", got)
		}
		if got := w.Header().Get(writtenExemplarsHeader); got != "" {
			t.Errorf("expected no exemplars header for v1, got %q", got)
		}
	})

	t.Run("writeHeaders v2 unconfirmed stats", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.Samples = 10
		resp.Histograms = 5
		resp.Exemplars = 2
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV2MessageType, w)

		if got := w.Header().Get("Custom-Header"); got != "custom-value" {
			t.Errorf("expected Custom-Header to be custom-value, got %q", got)
		}
		if got := w.Header().Get(writtenSamplesHeader); got != "" {
			t.Errorf("expected no samples header for unconfirmed stats, got %q", got)
		}
		if got := w.Header().Get(writtenHistogramsHeader); got != "" {
			t.Errorf("expected no histograms header for unconfirmed stats, got %q", got)
		}
		if got := w.Header().Get(writtenExemplarsHeader); got != "" {
			t.Errorf("expected no exemplars header for unconfirmed stats, got %q", got)
		}
	})

	t.Run("writeHeaders v2 confirmed stats", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.Add(WriteResponseStats{
			Samples:    10,
			Histograms: 5,
			Exemplars:  2,
			confirmed:  true,
		})
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV2MessageType, w)

		expectedHeaders := map[string]string{
			"Custom-Header":                                "custom-value",
			"X-Prometheus-Remote-Write-Samples-Written":    "10",
			"X-Prometheus-Remote-Write-Histograms-Written": "5",
			"X-Prometheus-Remote-Write-Exemplars-Written":  "2",
		}

		for k, want := range expectedHeaders {
			if got := w.Header().Get(k); got != want {
				t.Errorf("header %q: want %q, got %q", k, want, got)
			}
		}
	})

	t.Run("writeHeaders v2 confirmed but negative stats", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.Add(WriteResponseStats{
			Samples:    -1,
			Histograms: 5,
			Exemplars:  2,
			confirmed:  true,
		})
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV2MessageType, w)

		if got := w.Header().Get("Custom-Header"); got != "custom-value" {
			t.Errorf("expected Custom-Header to be custom-value, got %q", got)
		}
		if got := w.Header().Get(writtenSamplesHeader); got != "" {
			t.Errorf("expected no samples header for invalid stats, got %q", got)
		}
	})

	t.Run("writeHeaders invalid message type", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.Add(NewConfirmedWriteResponseStats(10, 5, 2))
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteMessageType("invalid.message.type"), w)

		if got := w.Header().Get("Custom-Header"); got != "custom-value" {
			t.Errorf("expected Custom-Header to be custom-value, got %q", got)
		}
		if got := w.Header().Get(writtenSamplesHeader); got != "" {
			t.Errorf("expected no samples header for invalid message type, got %q", got)
		}
	})

	t.Run("writeHeaders nil receiver", func(t *testing.T) {
		var resp *WriteResponse
		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV2MessageType, w)
		if len(w.Header()) != 0 {
			t.Errorf("expected empty header map for nil receiver, got %v", w.Header())
		}
	})
}

func TestWriteResponseStats(t *testing.T) {
	t.Run("constructors and confirmed getter/setter", func(t *testing.T) {
		s1 := NewWriteResponseStats(1, 2, 3, false)
		if s1.Confirmed() {
			t.Error("expected confirmed to be false")
		}
		if s1.Samples != 1 || s1.Histograms != 2 || s1.Exemplars != 3 {
			t.Errorf("unexpected counts in s1: %+v", s1)
		}

		s2 := NewConfirmedWriteResponseStats(4, 5, 6)
		if !s2.Confirmed() {
			t.Error("expected confirmed to be true")
		}
		if s2.Samples != 4 || s2.Histograms != 5 || s2.Exemplars != 6 {
			t.Errorf("unexpected counts in s2: %+v", s2)
		}

		s2.SetConfirmed(false)
		if s2.Confirmed() {
			t.Error("expected confirmed to be false after SetConfirmed(false)")
		}
	})

	t.Run("Validate", func(t *testing.T) {
		tests := []struct {
			name    string
			stats   WriteResponseStats
			wantErr bool
		}{
			{
				name:    "all zeros",
				stats:   WriteResponseStats{},
				wantErr: false,
			},
			{
				name:    "positive counts",
				stats:   WriteResponseStats{Samples: 10, Histograms: 5, Exemplars: 1},
				wantErr: false,
			},
			{
				name:    "negative samples",
				stats:   WriteResponseStats{Samples: -1, Histograms: 5, Exemplars: 1},
				wantErr: true,
			},
			{
				name:    "negative histograms",
				stats:   WriteResponseStats{Samples: 10, Histograms: -1, Exemplars: 1},
				wantErr: true,
			},
			{
				name:    "negative exemplars",
				stats:   WriteResponseStats{Samples: 10, Histograms: 5, Exemplars: -1},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.stats.Validate()
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("NoDataWritten and AllSamples", func(t *testing.T) {
		empty := WriteResponseStats{}
		if !empty.NoDataWritten() {
			t.Error("expected NoDataWritten to be true for empty stats")
		}
		if got := empty.AllSamples(); got != 0 {
			t.Errorf("expected AllSamples 0, got %d", got)
		}

		populated := NewConfirmedWriteResponseStats(7, 3, 2)
		if populated.NoDataWritten() {
			t.Error("expected NoDataWritten to be false for populated stats")
		}
		if got := populated.AllSamples(); got != 10 {
			t.Errorf("expected AllSamples 10, got %d", got)
		}
	})

	t.Run("Add confirmation propagation", func(t *testing.T) {
		// unconfirmed + confirmed -> confirmed
		s := WriteResponseStats{Samples: 1}
		s.Add(NewConfirmedWriteResponseStats(2, 0, 0))
		if !s.Confirmed() {
			t.Error("expected confirmed to become true after adding confirmed stats")
		}
		if s.Samples != 3 {
			t.Errorf("expected Samples 3, got %d", s.Samples)
		}

		// confirmed + unconfirmed -> still confirmed
		s.Add(WriteResponseStats{Samples: 5})
		if !s.Confirmed() {
			t.Error("expected confirmed to remain true after adding unconfirmed stats")
		}
		if s.Samples != 8 {
			t.Errorf("expected Samples 8, got %d", s.Samples)
		}

		// unconfirmed + unconfirmed -> unconfirmed
		unconf1 := WriteResponseStats{Samples: 1}
		unconf2 := WriteResponseStats{Samples: 2}
		unconf1.Add(unconf2)
		if unconf1.Confirmed() {
			t.Error("expected confirmed to remain false")
		}
		if unconf1.Samples != 3 {
			t.Errorf("expected Samples 3, got %d", unconf1.Samples)
		}
	})
}

func TestParseWriteResponseStats(t *testing.T) {
	t.Run("all headers present and valid", func(t *testing.T) {
		resp := &http.Response{
			Header: http.Header{
				writtenSamplesHeader:    []string{"42"},
				writtenHistogramsHeader: []string{"13"},
				writtenExemplarsHeader:  []string{"7"},
			},
		}
		stats, err := parseWriteResponseStats(resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !stats.Confirmed() {
			t.Error("expected stats to be confirmed")
		}
		if stats.Samples != 42 || stats.Histograms != 13 || stats.Exemplars != 7 {
			t.Errorf("unexpected stats: %+v", stats)
		}
	})

	t.Run("subset of headers present", func(t *testing.T) {
		resp := &http.Response{
			Header: http.Header{
				writtenSamplesHeader: []string{"20"},
			},
		}
		stats, err := parseWriteResponseStats(resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !stats.Confirmed() {
			t.Error("expected stats to be confirmed when at least one header is present")
		}
		if stats.Samples != 20 || stats.Histograms != 0 || stats.Exemplars != 0 {
			t.Errorf("unexpected stats: %+v", stats)
		}
	})

	t.Run("no stats headers present", func(t *testing.T) {
		resp := &http.Response{
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
		}
		stats, err := parseWriteResponseStats(resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.Confirmed() {
			t.Error("expected stats to be unconfirmed when no stats headers are present")
		}
		if stats.Samples != 0 || stats.Histograms != 0 || stats.Exemplars != 0 {
			t.Errorf("unexpected stats: %+v", stats)
		}
	})

	t.Run("invalid header value returns error", func(t *testing.T) {
		resp := &http.Response{
			Header: http.Header{
				writtenSamplesHeader: []string{"not-a-number"},
			},
		}
		stats, err := parseWriteResponseStats(resp)
		if err == nil {
			t.Fatal("expected error for non-integer header value, got nil")
		}
		if !stats.Confirmed() {
			t.Error("expected confirmed to be true because header was present")
		}
		if stats.Samples != 0 {
			t.Errorf("expected Samples 0 on parse failure, got %d", stats.Samples)
		}
	})
}
