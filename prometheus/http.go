// Copyright 2014 Prometheus Team
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
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	instLabels = []string{"handler", "method", "code"}

	reqCnt = NewCounterVec(
		CounterOpts{
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests made.",
		},
		instLabels,
	)

	reqDur = NewSummaryVec(
		SummaryOpts{
			Subsystem: "http",
			Name:      "request_duration_microseconds",
			Help:      "The HTTP request latencies in microseconds.",
		},
		instLabels,
	)

	reqSz = NewSummaryVec(
		SummaryOpts{
			Subsystem: "http",
			Name:      "request_size_bytes",
			Help:      "The HTTP request sizes in bytes.",
		},
		instLabels,
	)

	resSz = NewSummaryVec(
		SummaryOpts{
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "The HTTP response sizes in bytes.",
		},
		instLabels,
	)
)

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
// registers four metric vector collectors (if not already done) and reports
// http metrics to the (newly or already) registered collectors:
// http_requests_total (CounterVec), http_request_duration_microseconds
// (SummaryVec), http_request_size_bytes (SummaryVec), http_response_size_bytes
// (SummaryVec). Each has three labels: handler, method, code. The value of the
// handler label is set by the handlerName parameter of this function.
func InstrumentHandler(handlerName string, handler http.Handler) http.HandlerFunc {
	return InstrumentHandlerFunc(handlerName, handler.ServeHTTP)
}

// InstrumentHandlerFunc wraps the given function for instrumentation. It
// registers four metric vector collectors (if not already done) and reports
// http metrics to the (newly or already) registered collectors:
// http_requests_total (CounterVec), http_request_duration_microseconds
// (SummaryVec), http_request_size_bytes (SummaryVec), http_response_size_bytes
// (SummaryVec). Each has three labels: handler, method, code. The value of the
// handler label is set by the handlerName parameter of this function.
func InstrumentHandlerFunc(handlerName string, handlerFunc func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	regReqCnt := MustRegisterOrGet(reqCnt).(*CounterVec)
	regReqDur := MustRegisterOrGet(reqDur).(*SummaryVec)
	regReqSz := MustRegisterOrGet(reqSz).(*SummaryVec)
	regResSz := MustRegisterOrGet(resSz).(*SummaryVec)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()

		delegate := &responseWriterDelegator{ResponseWriter: w}
		out := make(chan int)
		go computeApproximateRequestSize(r, out)
		handlerFunc(delegate, r)

		elapsed := float64(time.Since(now)) / float64(time.Microsecond)

		method := sanitizeMethod(r.Method)
		code := sanitizeCode(delegate.status)
		regReqCnt.WithLabelValues(handlerName, method, code).Inc()
		regReqDur.WithLabelValues(handlerName, method, code).Observe(elapsed)
		regResSz.WithLabelValues(handlerName, method, code).Observe(float64(delegate.written))
		regReqSz.WithLabelValues(handlerName, method, code).Observe(float64(<-out))
	})
}

func computeApproximateRequestSize(r *http.Request, out chan int) {
	s := len(r.Method)
	if r.URL != nil {
		s += len(r.URL.String())
	}
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
	written         int
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
	r.written += n
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
