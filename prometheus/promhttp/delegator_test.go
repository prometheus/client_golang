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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func (rw *responseWriter) Write(_ []byte) (int, error) {
	return 0, nil
}

func (rw *responseWriter) WriteHeader(_ int) {
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

// failingReader always fails without producing any bytes.
type failingReader struct{}

func (f *failingReader) Read(p []byte) (int, error) {
	return 0, errors.New("upstream failed")
}

// TestReaderFromZeroByteFailDoesNotLockStatus200 verifies that when
// ReadFrom fails with zero bytes transferred, the HTTP status code is
// not locked to 200. The handler should be able to write an alternative
// status code (e.g. 502) via http.Error.
// This is a regression test for https://github.com/prometheus/client_golang/issues/2155.
func TestReaderFromZeroByteFailDoesNotLockStatus200(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(w, &failingReader{}); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})

	// Use httptest.NewServer to get a full http.ResponseWriter including io.ReaderFrom.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap with our delegator to test the ReadFrom behavior.
		delegated := newDelegator(w, nil)
		h.ServeHTTP(delegated, r)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected status 502 Bad Gateway, got %d", resp.StatusCode)
	}
}

// TestReaderFromSuccessWrites200 verifies that when ReadFrom succeeds
// and writes bytes, the HTTP status code is correctly set to 200.
func TestReaderFromSuccessWrites200(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(w, strings.NewReader("hello world")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delegated := newDelegator(w, nil)
		h.ServeHTTP(delegated, r)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello world" {
		t.Errorf("expected body %q, got %q", "hello world", string(body))
	}
}
