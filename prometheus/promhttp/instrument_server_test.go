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

// Package promhttp contains functions to create http.Handler instances to
// expose Prometheus metrics via HTTP. In later versions of this package, it
// will also contain tooling to instrument instances of http.Handler and
// http.RoundTripper.
//
// promhttp.Handler acts on the prometheus.DefaultGatherer. With HandlerFor,
// you can create a handler for a custom registry or anything that implements
// the Gatherer interface. It also allows to create handlers that act
// differently on errors or allow to log errors.
package promhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareWrapsFirstToLast(t *testing.T) {
	order := []int{}
	first := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 0)

			next.ServeHTTP(w, r)

			order = append(order, 6)
		})
	}

	second := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 1)

			next.ServeHTTP(w, r)

			order = append(order, 5)
		})
	}

	third := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, 2)

			next.ServeHTTP(w, r)

			order = append(order, 4)
		})
	}

	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, 3)
	})

	handler := NewServer("testHandler", wrapped, SetMiddleware(first, second, third))

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	handler.ServeHTTP(rec, req)

	for want, got := range order {
		if want != got {
			t.Fatalf("wanted %d, got %d", want, got)
		}
	}
}
