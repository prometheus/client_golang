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
func ClientTrace(obs prometheus.ObserverVec, c httpClient) httpClient {
	// The supplied histogram NEEDS a label for the httptrace event.
	// TODO: Using `event` for now, but any other name is acceptable.
	wc := &Client{
		c: c,
	}

	// TODO: Check for `event` label on histogram.

	wc.middleware = func(r *http.Request) (*http.Response, error) {
		var (
			host  = r.URL.Host
			start = time.Now()
		)

		trace := &httptrace.ClientTrace{
			DNSStart: func(_ httptrace.DNSStartInfo) {
				obs.WithLabelValues(host, "DNSStart").Observe(time.Since(start).Seconds())
			},
			DNSDone: func(_ httptrace.DNSDoneInfo) {
				obs.WithLabelValues(host, "DNSDone").Observe(time.Since(start).Seconds())
			},
			ConnectStart: func(_, _ string) {
				obs.WithLabelValues(host, "ConnectStart").Observe(time.Since(start).Seconds())
			},
			ConnectDone: func(net, addr string, err error) {
				if err != nil {
					return
				}
				obs.WithLabelValues(host, "ConnectDone").Observe(time.Since(start).Seconds())
			},
			GotConn: func(_ httptrace.GotConnInfo) {
				obs.WithLabelValues(host, "GotConn").Observe(time.Since(start).Seconds())
			},
			GotFirstResponseByte: func() {
				obs.WithLabelValues(host, "GotFirstResponseByte").Observe(time.Since(start).Seconds())
			},
		}
		r = r.WithContext(httptrace.WithClientTrace(context.Background(), trace))

		return wc.Do(r)
	}

	return wc
}

// InFlight is middleware that instruments number of open requests partitioned
// by http client and request host.
var InFlight = func(gauge prometheus.Gauge, c httpClient) httpClient {
	wc := &Client{
		c: c,
	}
	wc.middleware = func(r *http.Request) (*http.Response, error) {
		gauge.Inc()
		resp, err := wc.Do(r)
		gauge.Dec()
		return resp, err
	}
	return wc
}
