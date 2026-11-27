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
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
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

type readFromFunc func(io.Reader) (int64, error)

func (f readFromFunc) ReadFrom(r io.Reader) (int64, error) { return f(r) }

func TestReaderFromDelegatorExposesFileBeforeFirstRead(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "body")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	copyFile := readFromFunc(func(r io.Reader) (int64, error) {
		// After an outside flush TCP can use sendfile without a buffered read.
		if lr, ok := r.(*io.LimitedReader); ok {
			r = lr.R
		}
		conn, ok := r.(syscall.Conn)
		if !ok {
			t.Fatal("file descriptor is hidden")
		}
		_, err := conn.SyscallConn()
		return 0, err
	})
	w := struct {
		http.ResponseWriter
		io.ReaderFrom
	}{httptest.NewRecorder(), copyFile}
	d := newDelegator(w, func(int) { t.Error("descriptor lookup recorded a header") })
	for _, src := range []io.Reader{file, io.LimitReader(file, 1)} {
		if _, err := d.(io.ReaderFrom).ReadFrom(src); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReaderFromDelegatorObservesFirstRead(t *testing.T) {
	src := strings.NewReader("abcdef")
	limited := &io.LimitedReader{R: src, N: 4}
	observations := 0
	called := false
	copyBody := readFromFunc(func(r io.Reader) (int64, error) {
		called = true
		// A read without data must leave the header open.
		_, _ = r.Read(nil)
		if observations != 0 {
			t.Fatal("observed the header before reading any data")
		}
		var first [1]byte
		n, err := io.ReadFull(r, first[:])
		if err != nil {
			t.Fatal(err)
		}
		if observations != 2 {
			t.Errorf("got %d header observations after the first byte, want 2", observations)
		}
		// The TCP connection unwraps one LimitedReader to find the source.
		if lr, ok := r.(*io.LimitedReader); !ok || lr.R != src {
			t.Error("the original source is hidden after the first read")
		}
		rest, err := io.Copy(io.Discard, r)
		return int64(n) + rest, err
	})
	w := struct {
		http.ResponseWriter
		io.ReaderFrom
	}{httptest.NewRecorder(), copyBody}
	observe := func(int) { observations++ }
	d := newDelegator(newDelegator(w, observe), observe)
	if n, err := d.(io.ReaderFrom).ReadFrom(limited); n != 4 || err != nil || limited.N != 0 {
		t.Errorf("ReadFrom returned (%d, %v) and left a limit of %d, want (4, nil) and 0", n, err, limited.N)
	}
	if !called {
		t.Error("the wrapped writer's ReadFrom was never called")
	}
}

type readFunc func([]byte) (int, error)

func (f readFunc) Read(p []byte) (int, error) { return f(p) }

func TestReaderFromDelegatorPreservesChangesMadeDuringRead(t *testing.T) {
	for _, n := range []int{0, 1} {
		w := struct {
			http.ResponseWriter
			io.ReaderFrom
		}{httptest.NewRecorder(), new(bytes.Buffer)}
		d := newDelegator(w, func(int) {})
		tail := strings.NewReader("tail")
		lr := &io.LimitedReader{N: 1}
		lr.R = readFunc(func(p []byte) (int, error) {
			d.WriteHeader(http.StatusCreated)
			lr.R = tail
			p[0] = 'x'
			return n, io.EOF
		})
		if _, err := d.(io.ReaderFrom).ReadFrom(lr); err != nil {
			t.Fatal(err)
		}
		if lr.R != tail {
			t.Errorf("after a read of %d bytes, the caller's replacement reader was lost", n)
		}
		if got := d.Status(); got != http.StatusCreated {
			t.Errorf("after a read of %d bytes, got status %d, want 201", n, got)
		}
	}
}

func TestReaderFromDelegator(t *testing.T) {
	for _, tc := range []struct {
		name       string
		src        io.Reader
		wantStatus int
		wantBody   string
	}{
		{name: "error before data", src: iotest.ErrReader(errors.New("source failed")), wantStatus: http.StatusBadGateway},
		{name: "empty source", src: strings.NewReader(""), wantStatus: http.StatusNoContent},
		{name: "short source", src: strings.NewReader("short"), wantStatus: http.StatusOK, wantBody: "short"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type result struct {
				status          int
				observed        []int
				copied, written int64
			}
			results := make(chan result, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				var res result
				observe := func(code int) {
					res.observed = append(res.observed, code)
				}
				// The outer delegator takes its status from the copy result.
				d := newDelegator(newDelegator(w, observe), nil)
				// A handler that reports a failed or empty copy with its own status.
				n, err := d.(io.ReaderFrom).ReadFrom(tc.src)
				switch {
				case err != nil:
					d.WriteHeader(http.StatusBadGateway)
				case n == 0:
					d.WriteHeader(http.StatusNoContent)
				}
				res.status, res.copied, res.written = d.Status(), n, d.Written()
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
		})
	}
}
