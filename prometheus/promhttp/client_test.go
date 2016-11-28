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
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMiddlewareWrapsFirstToLast(t *testing.T) {
	order := []int{}
	first := func(r *http.Request, next Middleware) (*http.Response, error) {
		order = append(order, 0)

		resp, err := next(r)

		order = append(order, 3)
		return resp, err
	}

	second := func(req *http.Request, next Middleware) (*http.Response, error) {
		order = append(order, 1)

		return next(req)
	}

	third := func(req *http.Request, next Middleware) (*http.Response, error) {
		order = append(order, 2)
		return next(req)
	}

	promclient, err := NewClient(SetMiddleware(first, second, third))
	if err != nil {
		t.Fatalf("%v", err)
	}

	resp, err := promclient.Get("http://google.com")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer resp.Body.Close()

	for want, got := range order {
		if want != got {
			t.Fatalf("wanted %d, got %d", want, got)
		}
	}
}

func TestMiddlewareAPI(t *testing.T) {
	var (
		his   = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_histogram"})
		sum   = prometheus.NewSummary(prometheus.SummaryOpts{Name: "test_summary"})
		gauge = prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_gauge"})
	)

	obs := []prometheus.Observer{
		his,
		sum,
		prometheus.ObserverFunc(gauge.Set),
	}

	client := *http.DefaultClient
	client.Timeout = 300 * time.Millisecond

	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "inFlight"})
	inFlight := func(r *http.Request, next Middleware) (*http.Response, error) {
		log.Println("1st")
		inFlightGauge.Inc()

		resp, err := next(r)

		inFlightGauge.Dec()

		log.Println("last")
		return resp, err
	}

	counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_counter"})
	bytesSent := func(req *http.Request, next Middleware) (*http.Response, error) {
		counter.Add(42)
		log.Println("2nd")
		// counter.Add(float64(req.ContentLength))

		return next(req)
	}

	logging := func(req *http.Request, next Middleware) (*http.Response, error) {
		log.Println("3rd")
		return next(req)
	}

	promclient, err := NewClient(SetClient(client), SetObservers(obs), SetMiddleware(inFlight, bytesSent, logging))
	if err != nil {
		t.Fatalf("%v", err)
	}

	resp, err := promclient.Get("http://google.com")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer resp.Body.Close()
}
