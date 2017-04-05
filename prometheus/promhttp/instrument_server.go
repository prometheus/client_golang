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
	dto "github.com/prometheus/client_model/go"
)

// InstrumentHandlerInFlight accepts a Gauge and an http.Handler, returning a
// new http.Handler that wraps the supplied http.Handler. The provided Gauge
// must be registered in a registry in order to be used.
func InstrumentHandlerInFlight(g prometheus.Gauge, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Inc()
		next.ServeHTTP(w, r)
		g.Dec()
	})
}

// InstrumentHandlerDuration accepts an ObserverVec interface and an
// http.Handler, returning a new http.Handler that wraps the supplied
// http.Handler. The provided ObserverVec must be registered in a registry in
// order to be used.  If the wrapped http.Handler has not set a status code,
// i.e. the value is currently 0, the supplied ObserverVec will report a 200.
// The instance labels "code" and "method" are supported on the provided
// ObserverVec.  Note: Partitioning histograms is expensive.
func InstrumentHandlerDuration(obs prometheus.ObserverVec, next http.Handler) http.HandlerFunc {
	code, method := checkLabels(obs)

	if code {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			d := newDelegator(w)
			next.ServeHTTP(d, r)

			obs.With(labels(code, method, r.Method, d.Status())).Observe(time.Since(now).Seconds())
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		next.ServeHTTP(w, r)
		obs.With(labels(code, method, r.Method, 0)).Observe(time.Since(now).Seconds())
	})
}

// InstrumentHandlerCounter accepts an CounterVec interface and an
// http.Handler, returning a new http.Handler that wraps the supplied
// http.Handler. The provided CounterVec must be registered in a registry in
// order to be used.  If the wrapped http.Handler has not set a status code,
// i.e. the value is currently 0, the supplied counter will report a 200. The
// instance labels "code" and "method" are supported on the provided
// CounterVec.
func InstrumentHandlerCounter(counter *prometheus.CounterVec, next http.Handler) http.HandlerFunc {
	code, method := checkLabels(counter)

	if code {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			d := newDelegator(w)
			next.ServeHTTP(d, r)
			counter.With(labels(code, method, r.Method, d.Status())).Inc()
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		counter.With(labels(code, method, r.Method, 0)).Inc()
	})
}

// InstrumentHandlerRequestSize accepts an ObserverVec interface and an
// http.Handler, returning a new http.Handler that wraps the supplied
// http.Handler. The provided ObserverVec must be registered in a registry in
// order to be used.  If the wrapped http.Handler has not set a status code,
// i.e. the value is currently 0, the supplied ObserverVec will report a 200.
// The instance labels "code" and "method" are supported on the provided
// ObserverVec.  Note: Partitioning histograms is expensive.
func InstrumentHandlerRequestSize(obs prometheus.ObserverVec, next http.Handler) http.HandlerFunc {
	code, method := checkLabels(obs)

	if code {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			d := newDelegator(w)
			next.ServeHTTP(d, r)
			size := computeApproximateRequestSize(r)
			obs.With(labels(code, method, r.Method, d.Status())).Observe(float64(size))
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		size := computeApproximateRequestSize(r)
		obs.With(labels(code, method, r.Method, 0)).Observe(float64(size))
	})
}

// InstrumentHandlerResponseSize accepts an ObserverVec interface and an
// http.Handler, returning a new http.Handler that wraps the supplied
// http.Handler. The provided ObserverVec must be registered in a registry in
// order to be used.  If the wrapped http.Handler has not set a status code,
// i.e. the value is currently 0, the supplied ObserverVec will report a 200.
// The instance labels "code" and "method" are supported on the provided
// ObserverVec.  Note: Partitioning histograms is expensive.
func InstrumentHandlerResponseSize(obs prometheus.ObserverVec, next http.Handler) http.Handler {
	code, method := checkLabels(obs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := newDelegator(w)
		next.ServeHTTP(d, r)
		obs.With(labels(code, method, r.Method, d.Status())).Observe(float64(d.Written()))
	})
}

func checkLabels(c prometheus.Collector) (code bool, method bool) {
	var (
		desc *prometheus.Desc
		pm   dto.Metric
	)

	descc := make(chan *prometheus.Desc, 1)
	c.Describe(descc)

	select {
	case desc = <-descc:
	default:
		panic("no description provided by collector")
	}

	// TODO(beorn7): Remove this hacky way to check for instance labels
	// once Descriptors can have their dimensionality queried.
	if _, err := prometheus.NewConstMetric(desc, prometheus.UntypedValue, 0); err == nil {
		return
	} else if m, err := prometheus.NewConstMetric(desc, prometheus.UntypedValue, 0, ""); err == nil {
		if err := m.Write(&pm); err != nil {
			panic("error checking metric for labels")
		}

		name := *pm.Label[0].Name
		if name == "code" {
			code = true
		} else if name == "method" {
			method = true
		} else {
			panic("metric partitioned with non-supported labels")
		}
		return
	} else if m, err := prometheus.NewConstMetric(desc, prometheus.UntypedValue, 0, "", ""); err == nil {
		if err := m.Write(&pm); err != nil {
			panic("error checking metric for labels")
		}

		for _, label := range pm.Label {
			if *label.Name == "code" || *label.Name == "method" {
				continue
			}
			panic("metric partitioned with non-supported labels")
		}

		code = true
		method = true
		return
	} else {
		panic("metric partitioned with non-supported labels")
	}
}

func labels(code, method bool, reqMethod string, status int) prometheus.Labels {
	labels := prometheus.Labels{}

	if code {
		labels["code"] = sanitizeCode(status)
	}
	if method {
		labels["method"] = sanitizeMethod(reqMethod)
	}

	return labels
}

func computeApproximateRequestSize(r *http.Request) int {
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

// If the wrapped http.Handler has not set a status code, i.e. the value is
// currently 0, santizeCode will return 200, for consistency with behavior in
// the stdlib.
func sanitizeCode(s int) string {
	switch s {
	case 100:
		return "100"
	case 101:
		return "101"

	case 200, 0:
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

type delegator interface {
	Status() int
	Written() int64

	http.ResponseWriter
}

func newDelegator(w http.ResponseWriter) delegator {
	d := &responseWriterDelegator{ResponseWriter: w}

	_, cn := w.(http.CloseNotifier)
	_, fl := w.(http.Flusher)
	_, hj := w.(http.Hijacker)
	_, ps := w.(http.Pusher)
	_, rf := w.(io.ReaderFrom)
	if cn && fl && hj && rf && ps {
		return &fancyResponseWriterDelegator{d}
	}

	return d
}

type responseWriterDelegator struct {
	http.ResponseWriter

	handler, method string
	status          int
	written         int64
	wroteHeader     bool
}

func (r *responseWriterDelegator) Status() int {
	return r.status
}

func (r *responseWriterDelegator) Written() int64 {
	return r.written
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

func (r *fancyResponseWriterDelegator) Status() int {
	return r.status
}

func (r *fancyResponseWriterDelegator) Written() int64 {
	return r.written
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
