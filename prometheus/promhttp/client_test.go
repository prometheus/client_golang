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
	"net/http/httputil"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestClientMiddlewareAPI(t *testing.T) {
	client := *http.DefaultClient
	client.Timeout = 300 * time.Millisecond

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
		[]string{"event"},
	)

	promclient := InFlightC(inFlightGauge,
		CounterC(counter,
			ClientTrace(histVec, &client),
		),
	)

	resp, err := promclient.Get("http://google.com")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer resp.Body.Close()

	out, err := httputil.DumpResponse(resp, true)
	if err != nil {
		t.Fatalf("%v", err)
	}
}
