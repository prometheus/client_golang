// Copyright 2016 The Prometheus Authors
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

// Copyright (c) 2013, The Prometheus Authors
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// Package promhttp contains functions to create http.Handler instances to
// expose Prometheus metrics via HTTP. In later versions of this package, it
// will also contain tooling to instrument instances of http.Handler and
// http.RoundTripper.
//
// promhttp.Handler acts on the prometheus.DefaultGatherer. With HandlerFor,
// you can create a handler for a custom registry or anything that implements
// the Gatherer interface. It also allows to create handlers that act
// differently on errors or allow to log errors.
package promhttp

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func InFlight(g prometheus.Gauge, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Inc()
		next.ServeHTTP(w, r)
		g.Dec()
	})
}

func Latency(obs prometheus.ObserverVec, next http.Handler) http.Handler {
	// Maybe include path? Tell people to use a const label for now.
	labels := prometheus.Labels{}
	if _, err := obs.GetMetricWith(prometheus.Labels{"code": ""}); err == nil {
		labels = prometheus.Labels{"code": ""}
	}
	// TODO: How to delete these labels?
	// c.Delete(labels)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		delegate := &responseWriterDelegator{ResponseWriter: w}

		next.ServeHTTP(delegate, r)

		if _, prs := labels["code"]; prs {
			labels["code"] = sanitizeCode(delegate.status)
		}
		obs.With(labels).Observe(time.Since(now).Seconds())
	})
}

func Counter(counter *prometheus.CounterVec, next http.Handler) http.Handler {
	var (
		labels = prometheus.Labels{}
		err    error
	)

	// TODO(beorn7): Remove this hacky way to check for instance labels
	// once Descriptors can have their dimensionality queried.
	if _, err = counter.GetMetricWith(prometheus.Labels{"code": "", "method": ""}); err == nil {
		labels["code"] = ""
		labels["method"] = ""
	} else if _, err = counter.GetMetricWith(prometheus.Labels{"method": ""}); err == nil {
		labels["method"] = ""
	} else if _, err = counter.GetMetricWith(prometheus.Labels{"code": ""}); err == nil {
		labels["code"] = ""
	} else if _, err = counter.GetMetricWith(labels); err != nil {
		panic(`counter middleware requires instance labels "code", "method", or both "code" and "label".`)
	}
	// TODO: There is no delete exposed on interface Counter.
	// c.Delete(labels)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delegate := &responseWriterDelegator{ResponseWriter: w}

		next.ServeHTTP(delegate, r)

		if _, prs := labels["code"]; prs {
			labels["code"] = sanitizeCode(delegate.status)
		}
		if _, prs := labels["method"]; prs {
			labels["method"] = sanitizeMethod(r.Method)
		}
		counter.With(labels).Inc()
	})
}

func RequestSize(obs prometheus.Observer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		size := computeApproximateRequestSize(r)
		obs.Observe(float64(size))
	})
}

func ResponseSize(obs prometheus.Observer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delegate := &responseWriterDelegator{ResponseWriter: w}

		_, cn := w.(http.CloseNotifier)
		_, fl := w.(http.Flusher)
		_, hj := w.(http.Hijacker)
		_, ps := w.(http.Pusher)
		_, rf := w.(io.ReaderFrom)
		var rw http.ResponseWriter
		if cn && fl && hj && rf && ps {
			rw = &fancyResponseWriterDelegator{delegate}
		} else {
			rw = delegate
		}

		next.ServeHTTP(rw, r)
		obs.Observe(float64(delegate.written))
	})
}

func computeApproximateRequestSize(r *http.Request) int {
	// Get URL length in current go routine for avoiding a race condition.
	// HandlerFunc that runs in parallel may modify the URL.
	s := 0
	if r.URL != nil {
		s += len(r.URL.String())
	}

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
	return s
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

func (f *fancyResponseWriterDelegator) Push(target string, opts *http.PushOptions) error {
	return f.ResponseWriter.(http.Pusher).Push(target, opts)
}

func (f *fancyResponseWriterDelegator) ReadFrom(r io.Reader) (int64, error) {
	if !f.wroteHeader {
		f.WriteHeader(http.StatusOK)
	}
	n, err := f.ResponseWriter.(io.ReaderFrom).ReadFrom(r)
	f.written += n
	return n, err
}
