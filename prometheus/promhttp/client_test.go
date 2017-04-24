// Copyright 2017 The Prometheus Authors
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

package promhttp

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestClientMiddlewareAPI(t *testing.T) {
	client := http.DefaultClient
	client.Timeout = 1 * time.Second

	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "in_flight",
		Help: "In-flight count.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_counter",
			Help: "Counter.",
		},
		[]string{"code", "method"},
	)

	dnsLatencyVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dns_latency",
			Help:    "Trace dns latency histogram.",
			Buckets: []float64{.005, .01, .025, .05},
		},
		[]string{"event"},
	)
	tlsLatencyVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tls_latency",
			Help:    "Trace tls latency histogram.",
			Buckets: []float64{.05, .1, .25, .5},
		},
		[]string{"event"},
	)

	latencyVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "latency",
			Help:    "Overall latency histogram.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"code", "method"},
	)

	prometheus.MustRegister(counter, tlsLatencyVec, dnsLatencyVec, latencyVec, inFlightGauge)

	trace := &InstrumentTrace{
		DNSStart: func(t float64) {
			dnsLatencyVec.WithLabelValues("DNSStart")
		},
		DNSDone: func(t float64) {
			dnsLatencyVec.WithLabelValues("DNSDone")
		},
		TLSHandshakeStart: func(t float64) {
			tlsLatencyVec.WithLabelValues("TLSHandshakeStart")
		},
		TLSHandshakeDone: func(t float64) {
			tlsLatencyVec.WithLabelValues("TLSHandshakeDone")
		},
	}

	client.Transport = InstrumentRoundTripperInFlight(inFlightGauge,
		InstrumentRoundTripperCounter(counter,
			InstrumentRoundTripperTrace(trace,
				InstrumentRoundTripperDuration(latencyVec, http.DefaultTransport),
			),
		),
	)

	resp, err := client.Get("http://google.com")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer resp.Body.Close()

	out, err := httputil.DumpResponse(resp, true)
	if err != nil {
		t.Fatalf("%v", err)
	}
	fmt.Println(string(out))
}
