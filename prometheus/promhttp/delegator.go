// Copyright 2017 The Prometheus Authors
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
	"bufio"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"syscall"
)

const (
	closeNotifier = 1 << iota
	flusher
	hijacker
	readerFrom
	pusher
)

type delegator interface {
	http.ResponseWriter

	Status() int
	Written() int64
}

type responseWriterDelegator struct {
	http.ResponseWriter

	status             int
	written            int64
	wroteHeader        bool
	observeWriteHeader func(int)
}

func (r *responseWriterDelegator) Status() int {
	return r.status
}

func (r *responseWriterDelegator) Written() int64 {
	return r.written
}

func (r *responseWriterDelegator) WriteHeader(code int) {
	r.recordHeader(code)
	// Forward every call so the wrapped writer can report a repeated final header.
	r.ResponseWriter.WriteHeader(code)
}

// Record the status and notify the observer without writing a header.
// During ReadFrom the wrapped writer writes the header itself.
func (r *responseWriterDelegator) recordHeader(code int) {
	if r.observeWriteHeader != nil && !r.wroteHeader {
		// Call the observer once, even if the handler calls WriteHeader again.
		r.observeWriteHeader(code)
	}
	r.status = code
	r.wroteHeader = true
}

func (r *responseWriterDelegator) Write(b []byte) (int, error) {
	// If applicable, call WriteHeader here so that observeWriteHeader is
	// handled appropriately.
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController get the underlying http.ResponseWriter,
// by implementing the [rwUnwrapper](https://cs.opensource.google/go/go/+/refs/tags/go1.21.4:src/net/http/responsecontroller.go;l=42-44) interface.
func (r *responseWriterDelegator) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

type (
	closeNotifierDelegator struct{ *responseWriterDelegator }
	flusherDelegator       struct{ *responseWriterDelegator }
	hijackerDelegator      struct{ *responseWriterDelegator }
	readerFromDelegator    struct{ *responseWriterDelegator }
	pusherDelegator        struct{ *responseWriterDelegator }
)

func (d closeNotifierDelegator) CloseNotify() <-chan bool {
	//nolint:staticcheck // Ignore SA1019. http.CloseNotifier is deprecated but we keep it here to not break existing users.
	return d.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (d flusherDelegator) Flush() {
	// If applicable, call WriteHeader here so that observeWriteHeader is
	// handled appropriately.
	if !d.wroteHeader {
		d.WriteHeader(http.StatusOK)
	}
	d.ResponseWriter.(http.Flusher).Flush()
}

func (d hijackerDelegator) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return d.ResponseWriter.(http.Hijacker).Hijack()
}

// On the read path, net/http writes the implicit 200 inside ReadFrom, just
// after the first read that returns data. This observer records it at that read
// and puts the source back. Package net looks through one io.LimitedReader
// to find a source for sendfile or splice.
type firstReadObserver struct {
	src io.Reader
	lr  *io.LimitedReader
	d   *responseWriterDelegator
}

func (o *firstReadObserver) Read(p []byte) (int, error) {
	// With nested delegators, src is another observer that puts its own
	// source back during Read. Restoring src afterward would undo that.
	o.lr.R = o.src
	n, err := o.src.Read(p)
	if n == 0 {
		// Keep watching the reader the caller left in place.
		o.src = o.lr.R
		o.lr.R = o
	} else if !o.d.wroteHeader {
		// The source may have set an explicit status during Read.
		o.d.recordHeader(http.StatusOK)
	}
	return n, err
}

type observedWriterTo struct {
	io.Reader
	src io.WriterTo
	d   *responseWriterDelegator
}

func (o observedWriterTo) WriteTo(w io.Writer) (int64, error) {
	if o.d.wroteHeader {
		return o.src.WriteTo(w)
	}
	return o.src.WriteTo(headerObservingWriter{w, o.d})
}

type observedConn struct {
	observedWriterTo
	conn syscall.Conn
}

func (o observedConn) SyscallConn() (syscall.RawConn, error) {
	// A descriptor copy bypasses Read and WriteTo. Let TCP use sendfile
	// once the first read has recorded the header.
	if !o.d.wroteHeader {
		return nil, errors.ErrUnsupported
	}
	return o.conn.SyscallConn()
}

type headerObservingWriter struct {
	io.Writer
	d *responseWriterDelegator
}

func (w headerObservingWriter) Write(p []byte) (int, error) {
	if !w.d.wroteHeader {
		w.d.recordHeader(http.StatusOK)
	}
	return w.Writer.Write(p)
}

func (d readerFromDelegator) ReadFrom(re io.Reader) (int64, error) {
	if !d.wroteHeader && d.observeWriteHeader != nil {
		// Waiting for ReadFrom to return would include the whole body copy
		// in the header timing. Reuse a caller's LimitedReader, as with
		// io.CopyN. A second one would hide the file from sendfile.
		lr, ok := re.(*io.LimitedReader)
		if !ok {
			lr = &io.LimitedReader{R: re, N: math.MaxInt64}
		}
		o := &firstReadObserver{src: lr.R, lr: lr, d: d.responseWriterDelegator}
		lr.R = o
		defer func() {
			// A copy without data leaves the observer in the caller's reader.
			if lr.R == o {
				lr.R = o.src
			}
		}()
		if wt, ok := re.(io.WriterTo); ok {
			// Let the wrapped writer choose Read or WriteTo. The source's
			// WriteTo can use different write sizes from a buffered copy.
			observed := observedWriterTo{lr, wt, d.responseWriterDelegator}
			if conn, ok := wt.(syscall.Conn); ok {
				re = observedConn{observed, conn}
			} else {
				re = observed
			}
		} else {
			re = lr
		}
	}
	n, err := d.ResponseWriter.(io.ReaderFrom).ReadFrom(re)
	d.written += n
	if !d.wroteHeader && n > 0 {
		// Without a header observer, use the copy result to record the
		// default status. A later Write would otherwise send a second header.
		d.recordHeader(http.StatusOK)
	}
	return n, err
}

func (d pusherDelegator) Push(target string, opts *http.PushOptions) error {
	return d.ResponseWriter.(http.Pusher).Push(target, opts)
}

var pickDelegator = make([]func(*responseWriterDelegator) delegator, 32)

func init() {
	// TODO(beorn7): Code generation would help here.
	pickDelegator[0] = func(d *responseWriterDelegator) delegator { // 0
		return d
	}
	pickDelegator[closeNotifier] = func(d *responseWriterDelegator) delegator { // 1
		return closeNotifierDelegator{d}
	}
	pickDelegator[flusher] = func(d *responseWriterDelegator) delegator { // 2
		return flusherDelegator{d}
	}
	pickDelegator[flusher+closeNotifier] = func(d *responseWriterDelegator) delegator { // 3
		return struct {
			*responseWriterDelegator
			http.Flusher
			http.CloseNotifier
		}{d, flusherDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[hijacker] = func(d *responseWriterDelegator) delegator { // 4
		return hijackerDelegator{d}
	}
	pickDelegator[hijacker+closeNotifier] = func(d *responseWriterDelegator) delegator { // 5
		return struct {
			*responseWriterDelegator
			http.Hijacker
			http.CloseNotifier
		}{d, hijackerDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[hijacker+flusher] = func(d *responseWriterDelegator) delegator { // 6
		return struct {
			*responseWriterDelegator
			http.Hijacker
			http.Flusher
		}{d, hijackerDelegator{d}, flusherDelegator{d}}
	}
	pickDelegator[hijacker+flusher+closeNotifier] = func(d *responseWriterDelegator) delegator { // 7
		return struct {
			*responseWriterDelegator
			http.Hijacker
			http.Flusher
			http.CloseNotifier
		}{d, hijackerDelegator{d}, flusherDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[readerFrom] = func(d *responseWriterDelegator) delegator { // 8
		return readerFromDelegator{d}
	}
	pickDelegator[readerFrom+closeNotifier] = func(d *responseWriterDelegator) delegator { // 9
		return struct {
			*responseWriterDelegator
			io.ReaderFrom
			http.CloseNotifier
		}{d, readerFromDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[readerFrom+flusher] = func(d *responseWriterDelegator) delegator { // 10
		return struct {
			*responseWriterDelegator
			io.ReaderFrom
			http.Flusher
		}{d, readerFromDelegator{d}, flusherDelegator{d}}
	}
	pickDelegator[readerFrom+flusher+closeNotifier] = func(d *responseWriterDelegator) delegator { // 11
		return struct {
			*responseWriterDelegator
			io.ReaderFrom
			http.Flusher
			http.CloseNotifier
		}{d, readerFromDelegator{d}, flusherDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[readerFrom+hijacker] = func(d *responseWriterDelegator) delegator { // 12
		return struct {
			*responseWriterDelegator
			io.ReaderFrom
			http.Hijacker
		}{d, readerFromDelegator{d}, hijackerDelegator{d}}
	}
	pickDelegator[readerFrom+hijacker+closeNotifier] = func(d *responseWriterDelegator) delegator { // 13
		return struct {
			*responseWriterDelegator
			io.ReaderFrom
			http.Hijacker
			http.CloseNotifier
		}{d, readerFromDelegator{d}, hijackerDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[readerFrom+hijacker+flusher] = func(d *responseWriterDelegator) delegator { // 14
		return struct {
			*responseWriterDelegator
			io.ReaderFrom
			http.Hijacker
			http.Flusher
		}{d, readerFromDelegator{d}, hijackerDelegator{d}, flusherDelegator{d}}
	}
	pickDelegator[readerFrom+hijacker+flusher+closeNotifier] = func(d *responseWriterDelegator) delegator { // 15
		return struct {
			*responseWriterDelegator
			io.ReaderFrom
			http.Hijacker
			http.Flusher
			http.CloseNotifier
		}{d, readerFromDelegator{d}, hijackerDelegator{d}, flusherDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[pusher] = func(d *responseWriterDelegator) delegator { // 16
		return pusherDelegator{d}
	}
	pickDelegator[pusher+closeNotifier] = func(d *responseWriterDelegator) delegator { // 17
		return struct {
			*responseWriterDelegator
			http.Pusher
			http.CloseNotifier
		}{d, pusherDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[pusher+flusher] = func(d *responseWriterDelegator) delegator { // 18
		return struct {
			*responseWriterDelegator
			http.Pusher
			http.Flusher
		}{d, pusherDelegator{d}, flusherDelegator{d}}
	}
	pickDelegator[pusher+flusher+closeNotifier] = func(d *responseWriterDelegator) delegator { // 19
		return struct {
			*responseWriterDelegator
			http.Pusher
			http.Flusher
			http.CloseNotifier
		}{d, pusherDelegator{d}, flusherDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[pusher+hijacker] = func(d *responseWriterDelegator) delegator { // 20
		return struct {
			*responseWriterDelegator
			http.Pusher
			http.Hijacker
		}{d, pusherDelegator{d}, hijackerDelegator{d}}
	}
	pickDelegator[pusher+hijacker+closeNotifier] = func(d *responseWriterDelegator) delegator { // 21
		return struct {
			*responseWriterDelegator
			http.Pusher
			http.Hijacker
			http.CloseNotifier
		}{d, pusherDelegator{d}, hijackerDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[pusher+hijacker+flusher] = func(d *responseWriterDelegator) delegator { // 22
		return struct {
			*responseWriterDelegator
			http.Pusher
			http.Hijacker
			http.Flusher
		}{d, pusherDelegator{d}, hijackerDelegator{d}, flusherDelegator{d}}
	}
	pickDelegator[pusher+hijacker+flusher+closeNotifier] = func(d *responseWriterDelegator) delegator { // 23
		return struct {
			*responseWriterDelegator
			http.Pusher
			http.Hijacker
			http.Flusher
			http.CloseNotifier
		}{d, pusherDelegator{d}, hijackerDelegator{d}, flusherDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[pusher+readerFrom] = func(d *responseWriterDelegator) delegator { // 24
		return struct {
			*responseWriterDelegator
			http.Pusher
			io.ReaderFrom
		}{d, pusherDelegator{d}, readerFromDelegator{d}}
	}
	pickDelegator[pusher+readerFrom+closeNotifier] = func(d *responseWriterDelegator) delegator { // 25
		return struct {
			*responseWriterDelegator
			http.Pusher
			io.ReaderFrom
			http.CloseNotifier
		}{d, pusherDelegator{d}, readerFromDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[pusher+readerFrom+flusher] = func(d *responseWriterDelegator) delegator { // 26
		return struct {
			*responseWriterDelegator
			http.Pusher
			io.ReaderFrom
			http.Flusher
		}{d, pusherDelegator{d}, readerFromDelegator{d}, flusherDelegator{d}}
	}
	pickDelegator[pusher+readerFrom+flusher+closeNotifier] = func(d *responseWriterDelegator) delegator { // 27
		return struct {
			*responseWriterDelegator
			http.Pusher
			io.ReaderFrom
			http.Flusher
			http.CloseNotifier
		}{d, pusherDelegator{d}, readerFromDelegator{d}, flusherDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[pusher+readerFrom+hijacker] = func(d *responseWriterDelegator) delegator { // 28
		return struct {
			*responseWriterDelegator
			http.Pusher
			io.ReaderFrom
			http.Hijacker
		}{d, pusherDelegator{d}, readerFromDelegator{d}, hijackerDelegator{d}}
	}
	pickDelegator[pusher+readerFrom+hijacker+closeNotifier] = func(d *responseWriterDelegator) delegator { // 29
		return struct {
			*responseWriterDelegator
			http.Pusher
			io.ReaderFrom
			http.Hijacker
			http.CloseNotifier
		}{d, pusherDelegator{d}, readerFromDelegator{d}, hijackerDelegator{d}, closeNotifierDelegator{d}}
	}
	pickDelegator[pusher+readerFrom+hijacker+flusher] = func(d *responseWriterDelegator) delegator { // 30
		return struct {
			*responseWriterDelegator
			http.Pusher
			io.ReaderFrom
			http.Hijacker
			http.Flusher
		}{d, pusherDelegator{d}, readerFromDelegator{d}, hijackerDelegator{d}, flusherDelegator{d}}
	}
	pickDelegator[pusher+readerFrom+hijacker+flusher+closeNotifier] = func(d *responseWriterDelegator) delegator { // 31
		return struct {
			*responseWriterDelegator
			http.Pusher
			io.ReaderFrom
			http.Hijacker
			http.Flusher
			http.CloseNotifier
		}{d, pusherDelegator{d}, readerFromDelegator{d}, hijackerDelegator{d}, flusherDelegator{d}, closeNotifierDelegator{d}}
	}
}

func newDelegator(w http.ResponseWriter, observeWriteHeaderFunc func(int)) delegator {
	d := &responseWriterDelegator{
		ResponseWriter:     w,
		observeWriteHeader: observeWriteHeaderFunc,
	}

	id := 0
	//nolint:staticcheck // Ignore SA1019. http.CloseNotifier is deprecated but we keep it here to not break existing users.
	if _, ok := w.(http.CloseNotifier); ok {
		id += closeNotifier
	}
	if _, ok := w.(http.Flusher); ok {
		id += flusher
	}
	if _, ok := w.(http.Hijacker); ok {
		id += hijacker
	}
	if _, ok := w.(io.ReaderFrom); ok {
		id += readerFrom
	}
	if _, ok := w.(http.Pusher); ok {
		id += pusher
	}

	return pickDelegator[id](d)
}
