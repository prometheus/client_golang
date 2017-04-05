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
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMiddlewareAPI(t *testing.T) {
	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "inFlight",
		Help: "Gauge.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_counter",
			Help: "Counter.",
		},
		[]string{"code", "method"},
	)

	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "latency",
			Help:    "Histogram.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"code"},
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	prometheus.MustRegister(inFlightGauge, counter, histVec)

	chain := InstrumentHandlerInFlight(inFlightGauge,
		InstrumentHandlerCounter(counter,
			InstrumentHandlerDuration(histVec, handler),
		),
	)

	r, _ := http.NewRequest("GET", "www.example.com", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, r)
}

func ExampleMiddleware() {
	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "inFlight",
		Help: "Gauge.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_counter",
			Help: "Counter.",
		},
		[]string{"code", "method"},
	)

	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "latency",
			Help:    "Histogram.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"code"},
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	prometheus.MustRegister(inFlightGauge, counter, histVec)

	chain := InstrumentHandlerInFlight(inFlightGauge,
		InstrumentHandlerCounter(counter,
			InstrumentHandlerDuration(histVec, handler),
		),
	)

	http.Handle("/metrics", Handler())
	http.Handle("/", chain)

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatal(err)
	}
}

func ExampleHistogramByEndpoint() {
	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "inFlight",
		Help: "Gauge.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_counter",
			Help: "Counter.",
		},
		[]string{"code", "method"},
	)

	pushVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "latency",
			Help:        "Histogram.",
			Buckets:     []float64{.25, .5, 1, 2.5, 5, 10},
			ConstLabels: prometheus.Labels{"handler": "push"},
		},
		[]string{"code"},
	)
	pullVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "latency",
			Help:        "Histogram.",
			Buckets:     []float64{.005, .01, .025, .05},
			ConstLabels: prometheus.Labels{"handler": "pull"},
		},
		[]string{"code"},
	)

	pushHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Push"))
	})
	pullHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Pull"))
	})

	prometheus.MustRegister(inFlightGauge, counter, pullVec, pushVec)

	pushChain := InstrumentHandlerInFlight(inFlightGauge,
		InstrumentHandlerCounter(counter,
			InstrumentHandlerDuration(pushVec, pushHandler),
		),
	)

	pullChain := InstrumentHandlerInFlight(inFlightGauge,
		InstrumentHandlerCounter(counter,
			InstrumentHandlerDuration(pushVec, pullHandler),
		),
	)

	http.Handle("/metrics", Handler())
	http.Handle("/push", pushChain)
	http.Handle("/pull", pullChain)

	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatal(err)
	}
}
