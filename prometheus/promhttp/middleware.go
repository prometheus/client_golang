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
	"context"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ClientTrace adds middleware providing a histogram of outgoing request
// latencies, partitioned by http client, request host and httptrace event.
func ClientTrace(httpClientName string) middlewareFunc {
	hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   "outgoing_httptrace",
		Name:        "duration_seconds",
		ConstLabels: prometheus.Labels{"client": httpClientName},
		Help:        "Histogram of outgoing request latencies.",
		Buckets:     []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	},
		[]string{"host", "event"},
	)

	if err := prometheus.Register(hist); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			hist = are.ExistingCollector.(*prometheus.HistogramVec)
		} else {
			panic(err)
		}
	}

	return func(req *http.Request, next Middleware) (*http.Response, error) {
		var (
			host  = req.URL.Host
			start = time.Now()
		)

		trace := &httptrace.ClientTrace{
			DNSStart: func(_ httptrace.DNSStartInfo) {
				hist.WithLabelValues(host, "DNSStart").Observe(time.Since(start).Seconds())
			},
			DNSDone: func(_ httptrace.DNSDoneInfo) {
				hist.WithLabelValues(host, "DNSDone").Observe(time.Since(start).Seconds())
			},
			ConnectStart: func(_, _ string) {
				hist.WithLabelValues(host, "ConnectStart").Observe(time.Since(start).Seconds())
			},
			ConnectDone: func(net, addr string, err error) {
				if err != nil {
					return
				}
				hist.WithLabelValues(host, "ConnectDone").Observe(time.Since(start).Seconds())
			},
			GotConn: func(_ httptrace.GotConnInfo) {
				hist.WithLabelValues(host, "GotConn").Observe(time.Since(start).Seconds())
			},
			GotFirstResponseByte: func() {
				hist.WithLabelValues(host, "GotFirstResponseByte").Observe(time.Since(start).Seconds())
			},
		}
		req = req.WithContext(httptrace.WithClientTrace(context.Background(), trace))

		return next(req)
	}
}

// InFlight is middleware that instruments number of open requests partitioned
// by http client and request host.
var InFlight = func(httpClientName string) middlewareFunc {
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   "outgoing",
		Name:        "open_requests",
		ConstLabels: prometheus.Labels{"client": httpClientName},
		Help:        "Gauge of open outgoing requests.",
	},
		[]string{"host"},
	)

	if err := prometheus.Register(gauge); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			gauge = are.ExistingCollector.(*prometheus.GaugeVec)
		} else {
			panic(err)
		}
	}

	return func(req *http.Request, next Middleware) (*http.Response, error) {
		host := req.URL.Host
		gauge.WithLabelValues(host).Inc()

		resp, err := next(req)

		gauge.WithLabelValues(host).Dec()

		return resp, err
	}
}
