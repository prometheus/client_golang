// Copyright (c) 2013, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package exp

import (
	"fmt"
	"github.com/prometheus/client_golang"
	"github.com/prometheus/client_golang/metrics"
	"net/http"
	"strings"
	"time"
)

const (
	handler = "handler"
	code    = "code"
	method  = "method"
)

type (
	coarseMux struct {
		*http.ServeMux
	}

	handlerDelegator struct {
		delegate http.Handler
		pattern  string
	}
)

var (
	requestCounts    = metrics.NewCounter()
	requestDuration  = metrics.NewCounter()
	requestDurations = metrics.NewDefaultHistogram()
	requestBytes     = metrics.NewCounter()
	responseBytes    = metrics.NewCounter()

	// DefaultCoarseMux is a drop-in replacement for http.DefaultServeMux that
	// provides standardized telemetry for Go's standard HTTP handler registration
	// and dispatch API.
	//
	// The name is due to the coarse grouping of telemetry by (HTTP Method, HTTP Response Code,
	// and handler match pattern) triples.
	DefaultCoarseMux = newCoarseMux()
)

func (h handlerDelegator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rwd := NewResponseWriterDelegator(w)

	defer func() {
		duration := float64(time.Since(start) / time.Microsecond)
		status := rwd.Status()
		labels := map[string]string{handler: h.pattern, code: status, method: strings.ToLower(r.Method)}
		requestCounts.Increment(labels)
		requestDuration.IncrementBy(labels, duration)
		requestDurations.Add(labels, duration)
		requestBytes.IncrementBy(labels, float64(computeApproximateRequestSize(*r)))
		responseBytes.IncrementBy(labels, float64(rwd.BytesWritten))
	}()

	h.delegate.ServeHTTP(rwd, r)
}

func (h handlerDelegator) String() string {
	return fmt.Sprintf("handlerDelegator wrapping %s for %s", h.delegate, h.pattern)
}

// Handle registers a http.Handler to this CoarseMux.  See http.ServeMux.Handle.
func (m *coarseMux) handle(pattern string, handler http.Handler) {
	m.ServeMux.Handle(pattern, handlerDelegator{
		delegate: handler,
		pattern:  pattern,
	})
}

// Handle registers a handler to this CoarseMux.  See http.ServeMux.HandleFunc.
func (m *coarseMux) handleFunc(pattern string, handler http.HandlerFunc) {
	m.ServeMux.Handle(pattern, handlerDelegator{
		delegate: handler,
		pattern:  pattern,
	})
}

func newCoarseMux() *coarseMux {
	return &coarseMux{
		ServeMux: http.NewServeMux(),
	}
}

// Handle registers a http.Handler to DefaultCoarseMux.  See http.Handle.
func Handle(pattern string, handler http.Handler) {
	DefaultCoarseMux.handle(pattern, handler)
}

// HandleFunc registers a handler to DefaultCoarseMux.  See http.HandleFunc.
func HandleFunc(pattern string, handler http.HandlerFunc) {
	DefaultCoarseMux.handleFunc(pattern, handler)
}

func init() {
	registry.Register("http_requests_total", "A counter of the total number of HTTP requests made against the default multiplexor.", registry.NilLabels, requestCounts)
	registry.Register("http_request_durations_total_microseconds", "The total amount of time the default multiplexor has spent answering HTTP requests (microseconds).", registry.NilLabels, requestDuration)
	registry.Register("http_request_durations_microseconds", "The amounts of time the default multiplexor has spent answering HTTP requests (microseconds).", registry.NilLabels, requestDurations)
	registry.Register("http_request_bytes_total", "The total volume of content body sizes received (bytes).", registry.NilLabels, requestBytes)
	registry.Register("http_response_bytes_total", "The total volume of response payloads emitted (bytes).", registry.NilLabels, responseBytes)
}
