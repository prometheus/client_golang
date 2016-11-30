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

func TestClientTraceMiddleware(t *testing.T) {}

func ExampleMiddleware() {
	t := time.NewTicker(time.Second)
	defer t.Stop()

	client := *http.DefaultClient
	client.Timeout = 500 * time.Millisecond

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "request_length_counter",
		Help: "Counter of request length.",
	})
	prometheus.MustRegister(counter)
	customMiddleware := func(req *http.Request, next Middleware) (*http.Response, error) {
		counter.Add(float64(req.ContentLength))

		return next(req)
	}

	promclient, err := NewClient(
		SetClient(client),
		SetMiddleware(ClientTrace("example_client"), customMiddleware, InFlight("example_client")),
	)
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		if err := http.ListenAndServe(":3000", prometheus.Handler()); err != nil {
			log.Fatalln(err)
		}
	}()

	for {
		select {
		case <-t.C:
			log.Println("sending GET")
			resp, err := promclient.Get("http://example.com")
			if err != nil {
				log.Fatalln(err)
			}
			resp.Body.Close()
		}
	}
}
