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

package promhttp

import (
	"net/http"
	"testing"
	"time"
)

type responseWriter struct {
	flushErrorCalled       bool
	setWriteDeadlineCalled time.Time
	setReadDeadlineCalled  time.Time
}

func (rw *responseWriter) Header() http.Header {
	return nil
}

func (rw *responseWriter) Write(p []byte) (int, error) {
	return 0, nil
}

func (rw *responseWriter) WriteHeader(statusCode int) {
}

func (rw *responseWriter) FlushError() error {
	rw.flushErrorCalled = true

	return nil
}

func (rw *responseWriter) SetWriteDeadline(deadline time.Time) error {
	rw.setWriteDeadlineCalled = deadline

	return nil
}

func (rw *responseWriter) SetReadDeadline(deadline time.Time) error {
	rw.setReadDeadlineCalled = deadline

	return nil
}

// trackingResponseWriter records every status code passed to WriteHeader so we
// can verify that 1xx informational codes are forwarded but not recorded as the
// final status.
type trackingResponseWriter struct {
	codes []int
}

func (rw *trackingResponseWriter) Header() http.Header         { return http.Header{} }
func (rw *trackingResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (rw *trackingResponseWriter) WriteHeader(code int)        { rw.codes = append(rw.codes, code) }

// TestResponseWriterDelegatorInformationalStatusCode verifies that 1xx
// responses (e.g. 100 Continue) are forwarded to the underlying
// ResponseWriter but are NOT recorded as the final status code, mirroring
// the behaviour of net/http's own responseWriter. See GitHub issue #1772.
func TestResponseWriterDelegatorInformationalStatusCode(t *testing.T) {
	var observed []int
	observe := func(code int) { observed = append(observed, code) }

	inner := &trackingResponseWriter{}
	rwd := &responseWriterDelegator{
		ResponseWriter:     inner,
		observeWriteHeader: observe,
	}

	// Send 100 Continue first — should pass through to inner but not be
	// recorded as the final status or trigger observeWriteHeader.
	rwd.WriteHeader(http.StatusContinue)
	if rwd.wroteHeader {
		t.Error("wroteHeader must not be set after a 1xx informational response")
	}
	if rwd.status == http.StatusContinue {
		t.Error("status must not be set to 100 after an informational response")
	}
	if len(observed) != 0 {
		t.Errorf("observeWriteHeader must not be called for 1xx responses, got %v", observed)
	}
	if len(inner.codes) != 1 || inner.codes[0] != http.StatusContinue {
		t.Errorf("100 Continue must be forwarded to the inner ResponseWriter, got %v", inner.codes)
	}

	// Now send the real response.
	rwd.WriteHeader(http.StatusOK)
	if !rwd.wroteHeader {
		t.Error("wroteHeader must be set after the final response")
	}
	if rwd.status != http.StatusOK {
		t.Errorf("status must be 200 after the final response, got %d", rwd.status)
	}
	if len(observed) != 1 || observed[0] != http.StatusOK {
		t.Errorf("observeWriteHeader must be called once with 200, got %v", observed)
	}
}

func TestResponseWriterDelegatorUnwrap(t *testing.T) {
	w := &responseWriter{}
	rwd := &responseWriterDelegator{ResponseWriter: w}

	if rwd.Unwrap() != w {
		t.Error("unwrapped responsewriter must equal to the original responsewriter")
	}

	controller := http.NewResponseController(rwd)
	if err := controller.Flush(); err != nil || !w.flushErrorCalled {
		t.Error("FlushError must be propagated to the original responsewriter")
	}

	timeNow := time.Now()
	if err := controller.SetWriteDeadline(timeNow); err != nil || w.setWriteDeadlineCalled != timeNow {
		t.Error("SetWriteDeadline must be propagated to the original responsewriter")
	}

	if err := controller.SetReadDeadline(timeNow); err != nil || w.setReadDeadlineCalled != timeNow {
		t.Error("SetReadDeadline must be propagated to the original responsewriter")
	}
}
