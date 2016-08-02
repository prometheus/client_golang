// Copyright 2014 The Prometheus Authors
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

package prometheus

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/expfmt"
)

const (
	contentTypeHeader     = "Content-Type"
	contentLengthHeader   = "Content-Length"
	contentEncodingHeader = "Content-Encoding"
	acceptEncodingHeader  = "Accept-Encoding"
)

// Handler returns an HTTP handler for the DefaultRegistry. It is
// already instrumented with InstrumentHandler (using "prometheus" as handler
// name).
//
// Please note the issues described in the doc comment of InstrumentHandler. You
// might want to consider using UninstrumentedHandler instead. In fact, the
// instrumentation of the handler is DEPRECATED. In future versions of this
// package, the Handler function will return an uninstrumented handler, and the
// UninstrumentedHandler function will be removed.
//
// The returned Handler is using the same HandlerOpts as the Handler returned by
// UninstrumentedHandler. See there for details.
func Handler() http.Handler {
	return InstrumentHandler("prometheus", UninstrumentedHandler())
}

// UninstrumentedHandler returns an HTTP handler for the DefaultRegistry. The
// Handler uses the default HandlerOpts, i.e. report the first error as an HTTP
// error, no error logging, and compression if requested by the client.
//
// If you want to create a Handler for the DefaultRegistry with different
// HandlerOpts, create it with HandlerFor with the DefaultRegistry and your
// desired HandlerOpts.
//
// Note that in future versions of this package, UninstrumentedHandler will be
// replaced by Handler (which will then return an uninstrumented handler, see
// there for details).
func UninstrumentedHandler() http.Handler {
	return HandlerFor(DefaultRegistry, HandlerOpts{})
}

// HandlerFor returns an http.Handler for the provided Deliverer. The behavior ef
// the Handler is defined by the provided HandlerOpts. The Handler is NOT
// instrumented with InstrumentHandler.
func HandlerFor(reg Deliverer, opts HandlerOpts) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mfs, err := reg.Deliver()
		if err != nil {
			if opts.ErrorLog != nil {
				opts.ErrorLog.Println("error collecting metrics:", err)
			}
			switch opts.ErrorHandling {
			case PanicOnError:
				panic(err)
			case ContinueOnError:
				if len(mfs) == 0 {
					http.Error(w, "No metrics collected, last error:\n\n"+err.Error(), http.StatusInternalServerError)
					return
				}
			case HTTPErrorOnError:
				http.Error(w, "An error has occurred during metrics collection:\n\n"+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		contentType := expfmt.Negotiate(req.Header)
		buf := getBuf()
		defer giveBuf(buf)
		writer, encoding := decorateWriter(req, buf, opts.DisableCompression)
		enc := expfmt.NewEncoder(writer, contentType)
		var lastErr error
		for _, mf := range mfs {
			if err := enc.Encode(mf); err != nil {
				lastErr = err
				if opts.ErrorLog != nil {
					opts.ErrorLog.Println("error encoding metric family:", err)
				}
				switch opts.ErrorHandling {
				case PanicOnError:
					panic(err)
				case ContinueOnError:
					// Handled later.
				case HTTPErrorOnError:
					http.Error(w, "An error has occurred during metrics encoding:\n\n"+err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		if closer, ok := writer.(io.Closer); ok {
			closer.Close()
		}
		if lastErr != nil && buf.Len() == 0 {
			http.Error(w, "No metrics encoded, last error:\n\n"+err.Error(), http.StatusInternalServerError)
			return
		}
		header := w.Header()
		header.Set(contentTypeHeader, string(contentType))
		header.Set(contentLengthHeader, fmt.Sprint(buf.Len()))
		if encoding != "" {
			header.Set(contentEncodingHeader, encoding)
		}
		w.Write(buf.Bytes())
		// TODO(beorn7): Consider streaming serving of metrics.
	})
}

// HandlerErrorHandling defines how a Handler serving metrics will handle
// errors.
type HandlerErrorHandling int

// These constants cause handlers serving metrics to behave as described if
// errors are encountered.
const (
	// Serve an HTTP status code 500 upon the first error
	// encountered. Report the error message in the body.
	HTTPErrorOnError HandlerErrorHandling = iota
	// Ignore errors and try to serve as many metrics as possible.  However,
	// if no metrics can be served, serve an HTTP status code 500 and the
	// last error message in the body. Only use this in deliberate "best
	// effort" metrics collection scenarios. It is recommended to at least
	// log errors (by providing an ErrorLog in HandlerOpts) to not mask
	// errors completely.
	ContinueOnError
	// Panic upon the first error encountered (useful for "crash only" apps).
	PanicOnError
)

// Logger is the minimal interface HandlerOpts needs for logging. Note that
// log.Logger from the standard library implements this interface, and it is
// easy to implement by custom loggers, if they don't do so already anyway.
type Logger interface {
	Println(v ...interface{})
}

// HandlerOpts specifies options how to serve metrics via an http.Handler. The
// zero value of HandlerOpts is a reasonable default.
type HandlerOpts struct {
	// ErrorLog specifies an optional logger for errors collecting and
	// serving metrics. If nil, errors are not logged at all.
	ErrorLog Logger
	// ErrorHandling defines how errors are handled. Note that errors are
	// logged regardless of the configured ErrorHandling provided ErrorLog
	// is not nil.
	ErrorHandling HandlerErrorHandling
	// If DisableCompression is true, the handler will never compress the
	// response, even if requested by the client.
	DisableCompression bool
}

// decorateWriter wraps a writer to handle gzip compression if requested.  It
// returns the decorated writer and the appropriate "Content-Encoding" header
// (which is empty if no compression is enabled).
func decorateWriter(request *http.Request, writer io.Writer, compressionDisabled bool) (io.Writer, string) {
	if compressionDisabled {
		return writer, ""
	}
	header := request.Header.Get(acceptEncodingHeader)
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part := strings.TrimSpace(part)
		if part == "gzip" || strings.HasPrefix(part, "gzip;") {
			return gzip.NewWriter(writer), "gzip"
		}
	}
	return writer, ""
}

var instLabels = []string{"method", "code"}

type nower interface {
	Now() time.Time
}

type nowFunc func() time.Time

func (n nowFunc) Now() time.Time {
	return n()
}

var now nower = nowFunc(func() time.Time {
	return time.Now()
})

func nowSeries(t ...time.Time) nower {
	return nowFunc(func() time.Time {
		defer func() {
			t = t[1:]
		}()

		return t[0]
	})
}

// InstrumentHandler wraps the given HTTP handler for instrumentation. It
// registers four metric collectors (if not already done) and reports HTTP
// metrics to the (newly or already) registered collectors: http_requests_total
// (CounterVec), http_request_duration_microseconds (Summary),
// http_request_size_bytes (Summary), http_response_size_bytes (Summary). Each
// has a constant label named "handler" with the provided handlerName as
// value. http_requests_total is a metric vector partitioned by HTTP method
// (label name "method") and HTTP status code (label name "code").
//
// Note that InstrumentHandler has several issues:
//
// - It uses Summaries rather than Histograms. Summaries are not useful if
// aggregation across multiple instances is required.
//
// - It uses microseconds as unit, which is deprecated and should be replaced by
// seconds.
//
// - The size of the request is calculated in a separate goroutine. Since this
// calculator requires access to the request header, it creates a race with
// any writes to the header performed during request handling.
// httputil.ReverseProxy is a prominent example for a handler
// performing such writes.
//
// Upcoming versions of this package will provide ways of instrumenting HTTP
// handlers that are more flexible and have fewer issues. Consider this function
// DEPRECATED and prefer direct instrumentation in the meantime.
func InstrumentHandler(handlerName string, handler http.Handler) http.HandlerFunc {
	return InstrumentHandlerFunc(handlerName, handler.ServeHTTP)
}

// InstrumentHandlerFunc wraps the given function for instrumentation. It
// otherwise works in the same way as InstrumentHandler (and shares the same
// issues).
func InstrumentHandlerFunc(handlerName string, handlerFunc func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return InstrumentHandlerFuncWithOpts(
		SummaryOpts{
			Subsystem:   "http",
			ConstLabels: Labels{"handler": handlerName},
		},
		handlerFunc,
	)
}

// InstrumentHandlerWithOpts works like InstrumentHandler (and shares the same
// issues) but provides more flexibility (at the cost of a more complex call
// syntax). As InstrumentHandler, this function registers four metric
// collectors, but it uses the provided SummaryOpts to create them. However, the
// fields "Name" and "Help" in the SummaryOpts are ignored. "Name" is replaced
// by "requests_total", "request_duration_microseconds", "request_size_bytes",
// and "response_size_bytes", respectively. "Help" is replaced by an appropriate
// help string. The names of the variable labels of the http_requests_total
// CounterVec are "method" (get, post, etc.), and "code" (HTTP status code).
//
// If InstrumentHandlerWithOpts is called as follows, it mimics exactly the
// behavior of InstrumentHandler:
//
//     prometheus.InstrumentHandlerWithOpts(
//         prometheus.SummaryOpts{
//              Subsystem:   "http",
//              ConstLabels: prometheus.Labels{"handler": handlerName},
//         },
//         handler,
//     )
//
// Technical detail: "requests_total" is a CounterVec, not a SummaryVec, so it
// cannot use SummaryOpts. Instead, a CounterOpts struct is created internally,
// and all its fields are set to the equally named fields in the provided
// SummaryOpts.
func InstrumentHandlerWithOpts(opts SummaryOpts, handler http.Handler) http.HandlerFunc {
	return InstrumentHandlerFuncWithOpts(opts, handler.ServeHTTP)
}

// InstrumentHandlerFuncWithOpts works like InstrumentHandlerFunc (and shares
// the same issues) but provides more flexibility (at the cost of a more complex
// call syntax). See InstrumentHandlerWithOpts for details how the provided
// SummaryOpts are used.
func InstrumentHandlerFuncWithOpts(opts SummaryOpts, handlerFunc func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	reqCnt := NewCounterVec(
		CounterOpts{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			Name:        "requests_total",
			Help:        "Total number of HTTP requests made.",
			ConstLabels: opts.ConstLabels,
		},
		instLabels,
	)

	opts.Name = "request_duration_microseconds"
	opts.Help = "The HTTP request latencies in microseconds."
	reqDur := NewSummary(opts)

	opts.Name = "request_size_bytes"
	opts.Help = "The HTTP request sizes in bytes."
	reqSz := NewSummary(opts)

	opts.Name = "response_size_bytes"
	opts.Help = "The HTTP response sizes in bytes."
	resSz := NewSummary(opts)

	regReqCnt := MustRegisterOrGet(reqCnt).(*CounterVec)
	regReqDur := MustRegisterOrGet(reqDur).(Summary)
	regReqSz := MustRegisterOrGet(reqSz).(Summary)
	regResSz := MustRegisterOrGet(resSz).(Summary)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()

		delegate := &responseWriterDelegator{ResponseWriter: w}
		out := make(chan int)
		urlLen := 0
		if r.URL != nil {
			urlLen = len(r.URL.String())
		}
		go computeApproximateRequestSize(r, out, urlLen)

		_, cn := w.(http.CloseNotifier)
		_, fl := w.(http.Flusher)
		_, hj := w.(http.Hijacker)
		_, rf := w.(io.ReaderFrom)
		var rw http.ResponseWriter
		if cn && fl && hj && rf {
			rw = &fancyResponseWriterDelegator{delegate}
		} else {
			rw = delegate
		}
		handlerFunc(rw, r)

		elapsed := float64(time.Since(now)) / float64(time.Microsecond)

		method := sanitizeMethod(r.Method)
		code := sanitizeCode(delegate.status)
		regReqCnt.WithLabelValues(method, code).Inc()
		regReqDur.Observe(elapsed)
		regResSz.Observe(float64(delegate.written))
		regReqSz.Observe(float64(<-out))
	})
}

func computeApproximateRequestSize(r *http.Request, out chan int, s int) {
	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	// N.B. r.Form and r.MultipartForm are assumed to be included in r.URL.

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}
	out <- s
}

type responseWriterDelegator struct {
	http.ResponseWriter

	handler, method string
	status          int
	written         int64
	wroteHeader     bool
}

func (r *responseWriterDelegator) WriteHeader(code int) {
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseWriterDelegator) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

type fancyResponseWriterDelegator struct {
	*responseWriterDelegator
}

func (f *fancyResponseWriterDelegator) CloseNotify() <-chan bool {
	return f.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (f *fancyResponseWriterDelegator) Flush() {
	f.ResponseWriter.(http.Flusher).Flush()
}

func (f *fancyResponseWriterDelegator) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return f.ResponseWriter.(http.Hijacker).Hijack()
}

func (f *fancyResponseWriterDelegator) ReadFrom(r io.Reader) (int64, error) {
	if !f.wroteHeader {
		f.WriteHeader(http.StatusOK)
	}
	n, err := f.ResponseWriter.(io.ReaderFrom).ReadFrom(r)
	f.written += n
	return n, err
}

func sanitizeMethod(m string) string {
	switch m {
	case "GET", "get":
		return "get"
	case "PUT", "put":
		return "put"
	case "HEAD", "head":
		return "head"
	case "POST", "post":
		return "post"
	case "DELETE", "delete":
		return "delete"
	case "CONNECT", "connect":
		return "connect"
	case "OPTIONS", "options":
		return "options"
	case "NOTIFY", "notify":
		return "notify"
	default:
		return strings.ToLower(m)
	}
}

func sanitizeCode(s int) string {
	switch s {
	case 100:
		return "100"
	case 101:
		return "101"

	case 200:
		return "200"
	case 201:
		return "201"
	case 202:
		return "202"
	case 203:
		return "203"
	case 204:
		return "204"
	case 205:
		return "205"
	case 206:
		return "206"

	case 300:
		return "300"
	case 301:
		return "301"
	case 302:
		return "302"
	case 304:
		return "304"
	case 305:
		return "305"
	case 307:
		return "307"

	case 400:
		return "400"
	case 401:
		return "401"
	case 402:
		return "402"
	case 403:
		return "403"
	case 404:
		return "404"
	case 405:
		return "405"
	case 406:
		return "406"
	case 407:
		return "407"
	case 408:
		return "408"
	case 409:
		return "409"
	case 410:
		return "410"
	case 411:
		return "411"
	case 412:
		return "412"
	case 413:
		return "413"
	case 414:
		return "414"
	case 415:
		return "415"
	case 416:
		return "416"
	case 417:
		return "417"
	case 418:
		return "418"

	case 500:
		return "500"
	case 501:
		return "501"
	case 502:
		return "502"
	case 503:
		return "503"
	case 504:
		return "504"
	case 505:
		return "505"

	case 428:
		return "428"
	case 429:
		return "429"
	case 431:
		return "431"
	case 511:
		return "511"

	default:
		return strconv.Itoa(s)
	}
}
