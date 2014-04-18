// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"net/http"
	"time"
)

func InstrHandler(path string, hnd http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()

		delegate := &responseWriterDelegator{ResponseWriter: w}
		out := make(chan int, 1) // XXX
		go computeApproximateRequestSize(r, out)
		hnd.ServeHTTP(delegate, r)

		elapsed := float64(time.Since(now))

		method := sanitizeMethod(r.Method)
		code := sanitizeCode(delegate.status)
		reqCnt.Inc(path, method, code)
		reqDur.Observe(elapsed, path, method, code)
		resSz.Observe(float64(delegate.written), path, method, code)
		reqSz.Observe(float64(<-out), path, method, code)
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
	close(out)
}

type responseWriterDelegator struct {
	http.ResponseWriter

	handler, method string
	status          int
	written         int
}

func (r *responseWriterDelegator) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseWriterDelegator) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.written += n
	return n, err
}

func sanitizeMethod(m string) string {
	panic("unimpl")
}

func sanitizeCode(s int) string {
	panic("unimpl")
}

var instLabels = []string{"handler", "method", "code"}

var reqCnt = NewCounterVec(CounterVecDesc{
	Desc: Desc{
		Subsystem: "http",
		Name:      "requests_total",

		Help: "Total no. of HTTP requests made.",
	},
	Labels: instLabels,
})

var reqDur = NewSummaryVec(SummaryVecDesc{
	Desc: Desc{
		Subsystem: "http",
		Name:      "requests_duration_ms",

		Help: "The request latencies.",
	},
	Labels: instLabels,
})

var reqSz = NewSummaryVec(SummaryVecDesc{
	Desc: Desc{
		Subsystem: "http",
		Name:      "requests_size_bytes",

		Help: "The request sizes.",
	},
	Labels: instLabels,
})

var resSz = NewSummaryVec(SummaryVecDesc{
	Desc: Desc{
		Subsystem: "http",
		Name:      "response_size_bytes",

		Help: "The response sizes.",
	},
	Labels: instLabels,
})

func init() {
	MustRegister(reqCnt)
	MustRegister(reqDur)
	MustRegister(reqSz)
	MustRegister(resSz)
}
