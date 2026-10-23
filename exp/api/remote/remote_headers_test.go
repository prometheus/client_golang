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

	t.Run("writeHeaders v2", func(t *testing.T) {
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
			name:        "v2 unknown mode",
			contentType: "application/x-protobuf;proto=io.prometheus.write.v2.Request;mode=unknown",
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
		if got := contentTypeHeader(tc.msgType, tc.mode); got != tc.want {
			t.Errorf("contentTypeHeader(%q, %q) = %q, want %q", tc.msgType, tc.mode, got, tc.want)
		}
	}
}
