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

	t.Run("SetStats", func(t *testing.T) {
		resp := NewWriteResponse()
		if resp.Confirmed() {
			t.Error("expected newly created response to be unconfirmed")
		}

		stats := WriteResponseStats{
			Samples:    3,
			Histograms: 4,
			Exemplars:  5,
		}
		resp.SetStats(stats)
		if !resp.Confirmed() {
			t.Error("expected response to be confirmed after SetStats")
		}
		if diff := cmp.Diff(WriteResponseStats{
			Samples:    3,
			Histograms: 4,
			Exemplars:  5,
			confirmed:  true,
		}, resp.Stats(), cmpopts.IgnoreUnexported(WriteResponseStats{})); diff != "" {
			t.Errorf("stats mismatch (-want +got):\n%s", diff)
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
		resp.Add(WriteResponseStats{
			Samples:    10,
			Histograms: 5,
			Exemplars:  2,
		})
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV1MessageType, w)

		if got := w.Header().Get("Custom-Header"); got != "custom-value" {
			t.Errorf("expected Custom-Header to be custom-value, got %q", got)
		}
		if got := w.Header().Get(writtenSamplesHeader); got != "10" {
			t.Errorf("expected samples header 10, got %q", got)
		}
		if got := w.Header().Get(writtenHistogramsHeader); got != "5" {
			t.Errorf("expected histograms header 5, got %q", got)
		}
		if got := w.Header().Get(writtenExemplarsHeader); got != "2" {
			t.Errorf("expected exemplars header 2, got %q", got)
		}
	})

	t.Run("writeHeaders v2 unconfirmed stats", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV2MessageType, w)

		if got := w.Header().Get("Custom-Header"); got != "custom-value" {
			t.Errorf("expected Custom-Header to be custom-value, got %q", got)
		}
		if got := w.Header().Get(writtenSamplesHeader); got != "0" {
			t.Errorf("expected samples header 0 for unconfirmed v2 stats, got %q", got)
		}
		if got := w.Header().Get(writtenHistogramsHeader); got != "0" {
			t.Errorf("expected histograms header 0 for unconfirmed v2 stats, got %q", got)
		}
		if got := w.Header().Get(writtenExemplarsHeader); got != "0" {
			t.Errorf("expected exemplars header 0 for unconfirmed v2 stats, got %q", got)
		}
	})

	t.Run("writeHeaders v2 confirmed stats", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.Add(WriteResponseStats{
			Samples:    10,
			Histograms: 5,
			Exemplars:  2,
		})
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV2MessageType, w)

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

	t.Run("writeHeaders v2 confirmed but negative stats", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.Add(WriteResponseStats{
			Samples:    -1,
			Histograms: 5,
			Exemplars:  2,
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

	t.Run("writeHeaders v2 202 Accepted without confirmed stats omits written headers", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.SetStatusCode(http.StatusAccepted)
		resp.SetExtraHeader("Custom-Header", "custom-value")

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV2MessageType, w)

		if got := w.Header().Get("Custom-Header"); got != "custom-value" {
			t.Errorf("header %q: want %q, got %q", "Custom-Header", "custom-value", got)
		}
		for _, h := range []string{writtenSamplesHeader, writtenHistogramsHeader, writtenExemplarsHeader} {
			if got := w.Header().Get(h); got != "" {
				t.Errorf("expected header %q to be omitted on unconfirmed 202 Accepted, got %q", h, got)
			}
		}
		if !resp.Stats().Async {
			t.Errorf("expected Async to be true on 202 Accepted")
		}
		if resp.Stats().Confirmed() {
			t.Errorf("expected Confirmed() to be false when Add was not called")
		}
	})

	t.Run("writeHeaders v2 202 Accepted with confirmed stats sets written headers", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.SetStatusCode(http.StatusAccepted)
		resp.Add(WriteResponseStats{
			Samples:    3,
			Histograms: 1,
			Exemplars:  2,
		})

		w := httptest.NewRecorder()
		resp.writeHeaders(WriteV2MessageType, w)

		expectedHeaders := map[string]string{
			writtenSamplesHeader:    "3",
			writtenHistogramsHeader: "1",
			writtenExemplarsHeader:  "2",
		}
		for k, want := range expectedHeaders {
			if got := w.Header().Get(k); got != want {
				t.Errorf("header %q: want %q, got %q", k, want, got)
			}
		}
		if !resp.Stats().Async {
			t.Errorf("expected Async to be true on 202 Accepted")
		}
		if !resp.Stats().Confirmed() {
			t.Errorf("expected Confirmed() to be true after Add")
		}
	})

	t.Run("writeHeaders invalid message type", func(t *testing.T) {
		resp := NewWriteResponse()
		resp.Add(WriteResponseStats{
			Samples:    10,
			Histograms: 5,
			Exemplars:  2,
		})
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

func TestParseContentType(t *testing.T) {
	for _, tc := range []struct {
		name        string
		contentType string
		wantMsgType WriteMessageType
		wantMode    WriteMode
		wantErr     bool
	}{
		{
			name:        "empty returns error",
			contentType: "",
			wantErr:     true,
		},
		{
			name:        "v1 implicit without proto parameter",
			contentType: "application/x-protobuf",
			wantMsgType: WriteV1MessageType,
			wantMode:    WriteModeNone,
		},
		{
			name:        "v1 explicit",
			contentType: "application/x-protobuf;proto=prometheus.WriteRequest",
			wantMsgType: WriteV1MessageType,
			wantMode:    WriteModeNone,
		},
		{
			name:        "v1 rejects explicit mode=none",
			contentType: "application/x-protobuf;proto=prometheus.WriteRequest;mode=none",
			wantErr:     true,
		},
		{
			name:        "v1 rejects stateless mode",
			contentType: "application/x-protobuf;proto=prometheus.WriteRequest;mode=stateless",
			wantErr:     true,
		},
		{
			name:        "v1 implicit rejects stateless-st mode",
			contentType: "application/x-protobuf;mode=stateless-st",
			wantErr:     true,
		},
		{
			name:        "v2 default mode",
			contentType: "application/x-protobuf;proto=io.prometheus.write.v2.Request",
			wantMsgType: WriteV2MessageType,
			wantMode:    WriteModeNone,
		},
		{
			name:        "v2 explicit mode=none normalized to empty",
			contentType: "application/x-protobuf;proto=io.prometheus.write.v2.Request;mode=none",
			wantMsgType: WriteV2MessageType,
			wantMode:    WriteModeNone,
		},
		{
			name:        "v2 stateless mode",
			contentType: "application/x-protobuf;proto=io.prometheus.write.v2.Request;mode=stateless",
			wantMsgType: WriteV2MessageType,
			wantMode:    WriteModeStateless,
		},
		{
			name:        "v2 stateless-st mode",
			contentType: "application/x-protobuf;proto=io.prometheus.write.v2.Request;mode=stateless-st",
			wantMsgType: WriteV2MessageType,
			wantMode:    WriteModeStatelessST,
		},
		{
			name:        "v2 case-insensitive parameter names",
			contentType: "application/x-protobuf;PROTO=io.prometheus.write.v2.Request;MODE=stateless",
			wantMsgType: WriteV2MessageType,
			wantMode:    WriteModeStateless,
		},
		{
			name:        "v2 quoted parameter values",
			contentType: `application/x-protobuf;proto="io.prometheus.write.v2.Request";mode="stateless"`,
			wantMsgType: WriteV2MessageType,
			wantMode:    WriteModeStateless,
		},
		{
			name:        "v2 mode parameter before proto parameter",
			contentType: "application/x-protobuf;mode=stateless;proto=io.prometheus.write.v2.Request",
			wantMsgType: WriteV2MessageType,
			wantMode:    WriteModeStateless,
		},
		{
			name:        "v2 unknown mode",
			contentType: "application/x-protobuf;proto=io.prometheus.write.v2.Request;mode=unknown",
			wantErr:     true,
		},
		{
			name:        "v2 empty mode parameter",
			contentType: "application/x-protobuf;proto=io.prometheus.write.v2.Request;mode=",
			wantErr:     true,
		},
		{
			name:        "v2 empty quoted mode parameter",
			contentType: `application/x-protobuf;proto=io.prometheus.write.v2.Request;mode=""`,
			wantErr:     true,
		},
		{
			name:        "malformed parameter without value",
			contentType: "application/x-protobuf;proto",
			wantErr:     true,
		},
		{
			name:        "duplicate proto parameter",
			contentType: "application/x-protobuf;proto=prometheus.WriteRequest;proto=io.prometheus.write.v2.Request",
			wantErr:     true,
		},
		{
			name:        "duplicate mode parameter",
			contentType: "application/x-protobuf;proto=io.prometheus.write.v2.Request;mode=none;mode=stateless",
			wantErr:     true,
		},
		{
			name:        "unknown parameter",
			contentType: "application/x-protobuf;proto=io.prometheus.write.v2.Request;modes=stateless",
			wantErr:     true,
		},
		{
			name:        "unsupported media type",
			contentType: "application/json",
			wantErr:     true,
		},
		{
			name:        "unknown proto parameter",
			contentType: "application/x-protobuf;proto=unknown",
			wantErr:     true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotMsgType, gotMode, err := ParseContentType(tc.contentType)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil (msgType=%q, mode=%q)", tc.contentType, gotMsgType, gotMode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.contentType, err)
			}
			if gotMsgType != tc.wantMsgType {
				t.Errorf("msgType mismatch: want %q, got %q", tc.wantMsgType, gotMsgType)
			}
			if gotMode != tc.wantMode {
				t.Errorf("mode mismatch: want %q, got %q", tc.wantMode, gotMode)
			}
		})
	}
}

func TestContentTypeHeader(t *testing.T) {
	for _, tc := range []struct {
		msgType WriteMessageType
		mode    WriteMode
		want    string
	}{
		{
			msgType: WriteV1MessageType,
			mode:    WriteModeNone,
			want:    "application/x-protobuf",
		},
		{
			msgType: WriteV2MessageType,
			mode:    WriteModeNone,
			want:    "application/x-protobuf;proto=io.prometheus.write.v2.Request",
		},
		{
			msgType: WriteV2MessageType,
			mode:    WriteMode("none"),
			want:    "application/x-protobuf;proto=io.prometheus.write.v2.Request",
		},
		{
			msgType: WriteV2MessageType,
			mode:    WriteModeStateless,
			want:    "application/x-protobuf;proto=io.prometheus.write.v2.Request;mode=stateless",
		},
		{
			msgType: WriteV2MessageType,
			mode:    WriteModeStatelessST,
			want:    "application/x-protobuf;proto=io.prometheus.write.v2.Request;mode=stateless-st",
		},
	} {
		if got := ContentTypeHeader(tc.msgType, tc.mode); got != tc.want {
			t.Errorf("ContentTypeHeader(%q, %q) = %q, want %q", tc.msgType, tc.mode, got, tc.want)
		}
	}
}

func TestWriteResponseStats(t *testing.T) {
	t.Run("confirmed getter", func(t *testing.T) {
		s1 := WriteResponseStats{Samples: 1, Histograms: 2, Exemplars: 3}
		if s1.Confirmed() {
			t.Error("expected confirmed to be false")
		}
		if s1.Samples != 1 || s1.Histograms != 2 || s1.Exemplars != 3 {
			t.Errorf("unexpected counts in s1: %+v", s1)
		}

		s2 := WriteResponseStats{Samples: 4, Histograms: 5, Exemplars: 6, confirmed: true}
		if !s2.Confirmed() {
			t.Error("expected confirmed to be true")
		}
		if s2.Samples != 4 || s2.Histograms != 5 || s2.Exemplars != 6 {
			t.Errorf("unexpected counts in s2: %+v", s2)
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

		populated := WriteResponseStats{Samples: 7, Histograms: 3, Exemplars: 2, confirmed: true}
		if populated.NoDataWritten() {
			t.Error("expected NoDataWritten to be false for populated stats")
		}
		if got := populated.AllSamples(); got != 10 {
			t.Errorf("expected AllSamples 10, got %d", got)
		}
	})

	t.Run("Add confirmation propagation", func(t *testing.T) {
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

		// unconfirmed + confirmed -> confirmed
		s := WriteResponseStats{Samples: 1}
		s.Add(WriteResponseStats{Samples: 2, confirmed: true})
		if !s.Confirmed() {
			t.Error("expected confirmed to become true after adding confirmed stats")
		}
		if s.Samples != 3 {
			t.Errorf("expected Samples 3, got %d", s.Samples)
		}

		// confirmed + unconfirmed -> unconfirmed (s.confirmed = rs.confirmed per PRW retry semantics)
		s.Add(WriteResponseStats{Samples: 5})
		if s.Confirmed() {
			t.Error("expected confirmed to become false after adding unconfirmed stats")
		}
		if s.Samples != 8 {
			t.Errorf("expected Samples 8, got %d", s.Samples)
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

	t.Run("invalid header value returns error and unconfirmed stats", func(t *testing.T) {
		resp := &http.Response{
			Header: http.Header{
				writtenSamplesHeader: []string{"not-a-number"},
			},
		}
		stats, err := parseWriteResponseStats(resp)
		if err == nil {
			t.Fatal("expected error for non-integer header value, got nil")
		}
		if stats.Confirmed() {
			t.Error("expected confirmed to be false when header parsing fails and no other valid stats header exists")
		}
		if stats.Samples != 0 {
			t.Errorf("expected Samples 0 on parse failure, got %d", stats.Samples)
		}
	})

	t.Run("partial invalid header value with some valid headers retains confirmation", func(t *testing.T) {
		resp := &http.Response{
			Header: http.Header{
				writtenSamplesHeader:    []string{"not-a-number"},
				writtenHistogramsHeader: []string{"13"},
			},
		}
		stats, err := parseWriteResponseStats(resp)
		if err == nil {
			t.Fatal("expected error for non-integer header value, got nil")
		}
		if !stats.Confirmed() {
			t.Error("expected confirmed to be true when at least one valid stats header is parsed")
		}
		if stats.Samples != 0 {
			t.Errorf("expected Samples 0 on parse failure, got %d", stats.Samples)
		}
		if stats.Histograms != 13 {
			t.Errorf("expected Histograms 13, got %d", stats.Histograms)
		}
	})
}
