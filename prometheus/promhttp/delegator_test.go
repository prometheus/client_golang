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
	"testing/iotest"
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

// readFromSpy records the source that the ReadFrom of the wrapped
// ResponseWriter receives.
type readFromSpy struct {
	http.ResponseWriter
	src io.Reader
}

func (w *readFromSpy) ReadFrom(src io.Reader) (int64, error) {
	w.src = src
	return w.ResponseWriter.(io.ReaderFrom).ReadFrom(src)
}

func TestReaderFromDelegator(t *testing.T) {
	// Longer than the probe, so that data remains for the wrapped ReadFrom.
	long := strings.Repeat("x", 4096)
	for _, tc := range []struct {
		name          string
		src           io.Reader
		wantStatus    int
		wantBody      string
		wantDelegated bool
	}{
		{name: "error before data", src: iotest.ErrReader(errors.New("source failed")), wantStatus: http.StatusBadGateway},
		{name: "empty source", src: strings.NewReader(""), wantStatus: http.StatusNoContent},
		{name: "short source", src: strings.NewReader("short"), wantStatus: http.StatusOK, wantBody: "short"},
		{name: "long source", src: strings.NewReader(long), wantStatus: http.StatusOK, wantBody: long, wantDelegated: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type result struct {
				status                   int
				observed                 []int
				observedBeforeDelegation bool
				copied, written          int64
				delegatedSrc             io.Reader
			}
			results := make(chan result, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				var res result
				spy := &readFromSpy{ResponseWriter: w}
				d := newDelegator(spy, func(code int) {
					res.observed = append(res.observed, code)
					res.observedBeforeDelegation = spy.src == nil
				})
				// A handler that reports a failed or empty copy with its own status.
				n, err := d.(io.ReaderFrom).ReadFrom(tc.src)
				switch {
				case err != nil:
					d.WriteHeader(http.StatusBadGateway)
				case n == 0:
					d.WriteHeader(http.StatusNoContent)
				}
				res.status, res.copied, res.written, res.delegatedSrc = d.Status(), n, d.Written(), spy.src
				results <- res
			}))
			defer srv.Close()

			resp, err := http.Get(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			res := <-results

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("got response status %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if string(body) != tc.wantBody {
				t.Errorf("response body differs from the source: got %d bytes, want %d", len(body), len(tc.wantBody))
			}
			if res.status != tc.wantStatus {
				t.Errorf("got Status() %d, want %d", res.status, tc.wantStatus)
			}
			if len(res.observed) != 1 || res.observed[0] != tc.wantStatus {
				t.Errorf("got observed status %v, want [%d]", res.observed, tc.wantStatus)
			}
			if want := int64(len(tc.wantBody)); res.copied != want || res.written != want {
				t.Errorf("got %d from ReadFrom and Written() %d, want %d for both", res.copied, res.written, want)
			}
			if !tc.wantDelegated {
				// Nothing remains for the wrapped ReadFrom after the probe.
				if res.delegatedSrc != nil {
					t.Error("ReadFrom of the wrapped ResponseWriter was called")
				}
				return
			}
			// net/http can use sendfile only if it gets the source itself, not a wrapper.
			if res.delegatedSrc != tc.src {
				t.Error("ReadFrom of the wrapped ResponseWriter did not get the source itself")
			}
			// The status must be observed when the data starts to flow, not after the copy.
			if !res.observedBeforeDelegation {
				t.Error("status was observed after the call to the wrapped ReadFrom")
			}
		})
	}
}
