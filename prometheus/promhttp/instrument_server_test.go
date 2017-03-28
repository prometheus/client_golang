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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMiddlewareAPI(t *testing.T) {
	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "inFlight"})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_counter"},
		[]string{"code", "method"},
	)

	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"code"},
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	chain := InFlight(inFlightGauge,
		Counter(counter,
			Latency(histVec, handler),
		),
	)

	r, _ := http.NewRequest("GET", "www.example.com", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, r)
}
