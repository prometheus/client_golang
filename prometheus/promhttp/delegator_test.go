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

// readFromResponseWriter is an http.ResponseWriter implementing
// io.ReaderFrom the way net/http's response does: ReadFrom writes
// directly to the writer itself, bypassing any wrapper.
type readFromResponseWriter struct {
	header         http.Header
	codes          []int
	body           []byte
	readFromCalled bool
}

func (w *readFromResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *readFromResponseWriter) WriteHeader(code int) {
	w.codes = append(w.codes, code)
}

func (w *readFromResponseWriter) Write(b []byte) (int, error) {
	if len(w.codes) == 0 {
		w.WriteHeader(http.StatusOK)
	}
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *readFromResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	w.readFromCalled = true
	// struct{ io.Writer } hides this type's own ReadFrom from io.Copy.
	return io.Copy(struct{ io.Writer }{w}, r)
}

func TestReaderFromDelegatorErrorBeforeData(t *testing.T) {
	errSource := errors.New("source failed")
	w := &readFromResponseWriter{}
	var observed []int
	d := newDelegator(w, func(code int) { observed = append(observed, code) })

	n, err := d.(io.ReaderFrom).ReadFrom(iotest.ErrReader(errSource))
	if !errors.Is(err, errSource) {
		t.Fatalf("expected source error, got %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 bytes copied, got %d", n)
	}
	if len(w.codes) != 0 {
		t.Fatalf("no status must be committed before the source produces data, got %v", w.codes)
	}
	if len(observed) != 0 {
		t.Fatalf("observeWriteHeader must not fire before data, got %v", observed)
	}

	// The handler must still be able to send an error status afterwards.
	d.WriteHeader(http.StatusBadGateway)
	if len(w.codes) != 1 || w.codes[0] != http.StatusBadGateway {
		t.Fatalf("expected [502] to reach the client, got %v", w.codes)
	}
	if len(observed) != 1 || observed[0] != http.StatusBadGateway {
		t.Fatalf("expected observeWriteHeader to fire with 502, got %v", observed)
	}
	if d.Status() != http.StatusBadGateway {
		t.Fatalf("expected delegator status 502, got %d", d.Status())
	}
}

func TestReaderFromDelegatorCopiesData(t *testing.T) {
	for _, size := range []int{0, 5, readFromProbeLen, 3 * readFromProbeLen} {
		w := &readFromResponseWriter{}
		d := newDelegator(w, nil)
		payload := strings.Repeat("x", size)

		n, err := d.(io.ReaderFrom).ReadFrom(strings.NewReader(payload))
		if err != nil {
			t.Fatalf("size %d: unexpected error %v", size, err)
		}
		if n != int64(size) {
			t.Fatalf("size %d: expected %d bytes copied, got %d", size, size, n)
		}
		if string(w.body) != payload {
			t.Fatalf("size %d: body mismatch", size)
		}
		if d.Written() != int64(size) {
			t.Fatalf("size %d: expected Written()=%d, got %d", size, size, d.Written())
		}
		if size == 0 {
			if len(w.codes) != 0 {
				t.Fatalf("size 0: no status must be committed for an empty source, got %v", w.codes)
			}
			continue
		}
		if len(w.codes) != 1 || w.codes[0] != http.StatusOK {
			t.Fatalf("size %d: expected a single implicit 200, got %v", size, w.codes)
		}
		if d.Status() != http.StatusOK {
			t.Fatalf("size %d: expected delegator status 200, got %d", size, d.Status())
		}
	}
}
