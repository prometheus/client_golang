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
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func instrumentClient(client *http.Client) *prometheus.Registry {
	reg := prometheus.NewRegistry()

	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "client_in_flight_requests",
		Help: "A gauge of in-flight requests for the wrapped client.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "client_api_requests_total",
			Help: "A counter for requests from the wrapped client.",
		},
		[]string{"code", "method"},
	)

	dnsLatencyVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dns_duration_seconds",
			Help:    "Trace dns latency histogram.",
			Buckets: []float64{.005, .01, .025, .05},
		},
		[]string{"event"},
	)

	tlsLatencyVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tls_duration_seconds",
			Help:    "Trace tls latency histogram.",
			Buckets: []float64{.05, .1, .25, .5},
		},
		[]string{"event"},
	)

	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "A histogram of request latencies.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	reg.MustRegister(counter, tlsLatencyVec, dnsLatencyVec, histVec, inFlightGauge)

	trace := &InstrumentTrace{
		DNSStart: func(t float64) {
			dnsLatencyVec.WithLabelValues("dns_start").Observe(t)
		},
		DNSDone: func(t float64) {
			dnsLatencyVec.WithLabelValues("dns_done").Observe(t)
		},
		TLSHandshakeStart: func(t float64) {
			tlsLatencyVec.WithLabelValues("tls_handshake_start").Observe(t)
		},
		TLSHandshakeDone: func(t float64) {
			tlsLatencyVec.WithLabelValues("tls_handshake_done").Observe(t)
		},
	}

	client.Transport = InstrumentRoundTripperInFlight(inFlightGauge,
		InstrumentRoundTripperCounter(counter,
			InstrumentRoundTripperTrace(trace,
				InstrumentRoundTripperDuration(histVec, http.DefaultTransport),
			),
		),
	)	

	return reg
}

func TestClientMiddlewareAPI(t *testing.T) {
	client := http.DefaultClient
	client.Timeout = 1 * time.Second

	instrumentClient(client)

	resp, err := client.Get("http://google.com")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer resp.Body.Close()
}

func TestClientMiddlewareAPIWithRequestContext(t *testing.T) {
	client := http.DefaultClient
	client.Timeout = 1 * time.Second

	reg:= instrumentClient(client);

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	req, err := http.NewRequest("GET", backend.URL, nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	_, err = client.Do(req)

	assert.Nil(t, err)
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(mfs) < 3 {
		t.Fatalf("Expected 3 metric families, found %d", len(mfs))
	}
	for _, mf := range mfs {
		assert.True(t, len(mf.Metric)>0, fmt.Sprintf("metrics %s must not be empty", mf.GetName()))
	}
}

func TestClientMiddlewareAPIWithRequestContextTimeout(t *testing.T) {
	client := http.DefaultClient
	client.Timeout = 1 * time.Second

	instrumentClient(client)

	// slow responding server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
	}))
	defer backend.Close()

	req, err := http.NewRequest("GET", backend.URL, nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// fast timeouting client context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	_, err = client.Do(req)

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Sprintf("Get %s: context deadline exceeded", backend.URL), err.Error())
}

func ExampleInstrumentRoundTripperDuration() {
	client := http.DefaultClient
	client.Timeout = 1 * time.Second

	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "client_in_flight_requests",
		Help: "A gauge of in-flight requests for the wrapped client.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "client_api_requests_total",
			Help: "A counter for requests from the wrapped client.",
		},
		[]string{"code", "method"},
	)

	// dnsLatencyVec uses custom buckets based on expected dns durations.
	// It has an instance label "event", which is set in the
	// DNSStart and DNSDonehook functions defined in the
	// InstrumentTrace struct below.
	dnsLatencyVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dns_duration_seconds",
			Help:    "Trace dns latency histogram.",
			Buckets: []float64{.005, .01, .025, .05},
		},
		[]string{"event"},
	)

	// tlsLatencyVec uses custom buckets based on expected tls durations.
	// It has an instance label "event", which is set in the
	// TLSHandshakeStart and TLSHandshakeDone hook functions defined in the
	// InstrumentTrace struct below.
	tlsLatencyVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tls_duration_seconds",
			Help:    "Trace tls latency histogram.",
			Buckets: []float64{.05, .1, .25, .5},
		},
		[]string{"event"},
	)

	// histVec has no labels, making it a zero-dimensional ObserverVec.
	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "A histogram of request latencies.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{},
	)

	// Register all of the metrics in the standard registry.
	prometheus.MustRegister(counter, tlsLatencyVec, dnsLatencyVec, histVec, inFlightGauge)

	// Define functions for the available httptrace.ClientTrace hook
	// functions that we want to instrument.
	trace := &InstrumentTrace{
		DNSStart: func(t float64) {
			dnsLatencyVec.WithLabelValues("dns_start").Observe(t)
		},
		DNSDone: func(t float64) {
			dnsLatencyVec.WithLabelValues("dns_done").Observe(t)
		},
		TLSHandshakeStart: func(t float64) {
			tlsLatencyVec.WithLabelValues("tls_handshake_start").Observe(t)
		},
		TLSHandshakeDone: func(t float64) {
			tlsLatencyVec.WithLabelValues("tls_handshake_done").Observe(t)
		},
	}

	// Wrap the default RoundTripper with middleware.
	roundTripper := InstrumentRoundTripperInFlight(inFlightGauge,
		InstrumentRoundTripperCounter(counter,
			InstrumentRoundTripperTrace(trace,
				InstrumentRoundTripperDuration(histVec, http.DefaultTransport),
			),
		),
	)

	// Set the RoundTripper on our client.
	client.Transport = roundTripper

	resp, err := client.Get("http://google.com")
	if err != nil {
		log.Printf("error: %v", err)
	}
	defer resp.Body.Close()
}
