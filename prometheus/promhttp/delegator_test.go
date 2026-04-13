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

// trackingResponseWriter records every WriteHeader call so tests can assert
// which status codes were forwarded to the underlying ResponseWriter.
type trackingResponseWriter struct {
	responseWriter
	codes []int
}

func (rw *trackingResponseWriter) WriteHeader(code int) {
	rw.codes = append(rw.codes, code)
}

func TestResponseWriterDelegatorIgnores1xxStatus(t *testing.T) {
	t.Run("100 Continue does not become the final status", func(t *testing.T) {
		observed := 0
		w := &trackingResponseWriter{}
		rwd := &responseWriterDelegator{
			ResponseWriter: w,
			observeWriteHeader: func(code int) {
				observed = code
			},
		}

		// Simulate a handler that sends 100 Continue then writes a body
		// (which implicitly triggers a 200 OK).
		rwd.WriteHeader(http.StatusContinue)
		rwd.Write([]byte("hello"))

		if rwd.Status() != http.StatusOK {
			t.Errorf("expected status 200, got %d", rwd.Status())
		}
		if observed != http.StatusOK {
			t.Errorf("expected observeWriteHeader called with 200, got %d", observed)
		}
		// 100 must still have been forwarded to the underlying ResponseWriter.
		if len(w.codes) < 1 || w.codes[0] != http.StatusContinue {
			t.Errorf("expected 100 forwarded to underlying writer, got %v", w.codes)
		}
	})

	t.Run("explicit 200 after 100 Continue is recorded correctly", func(t *testing.T) {
		observed := 0
		w := &trackingResponseWriter{}
		rwd := &responseWriterDelegator{
			ResponseWriter: w,
			observeWriteHeader: func(code int) {
				observed = code
			},
		}

		rwd.WriteHeader(http.StatusContinue)
		rwd.WriteHeader(http.StatusOK)

		if rwd.Status() != http.StatusOK {
			t.Errorf("expected status 200, got %d", rwd.Status())
		}
		if observed != http.StatusOK {
			t.Errorf("expected observeWriteHeader called with 200, got %d", observed)
		}
	})
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
