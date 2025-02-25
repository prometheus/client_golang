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
		if len(resp.ExtraHeaders()) != 0 {
			t.Errorf("expected empty headers, got %v", resp.ExtraHeaders())
		}
	})

	t.Run("setters and getters", func(t *testing.T) {
		resp := NewWriteResponse()

		resp.SetStatusCode(http.StatusOK)
		if got := resp.StatusCode(); got != http.StatusOK {
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
		if got := resp.ExtraHeaders().Get("Test-Header"); got != "test-value" {
			t.Errorf("expected header value %q, got %q", "test-value", got)
		}
	})

	t.Run("set headers on response writer", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.Add(WriteResponseStats{
			Samples:    10,
			Histograms: 5,
			Exemplars:  2,
			confirmed:  true,
		})
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.SetHeaders(w)

		expectedHeaders := map[string]string{
			"Custom-Header": "custom-value",
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
}
