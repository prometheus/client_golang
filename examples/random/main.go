// Copyright 2015 Prometheus Team
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

// A simple example exposing fictional RPC latencies with different types of
// random distributions (uniform, normal, and exponential) as Prometheus
// metrics.
package main

import (
	"flag"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	addr          = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
	uniformDomain = flag.Float64("random.uniform.domain", 200, "The domain for the uniform distribution.")
	normDomain    = flag.Float64("random.exponential.domain", 200, "The domain for the normal distribution.")
	normMean      = flag.Float64("random.exponential.mean", 10, "The mean for the normal distribution.")
)

var (
	// Create a summary to track fictional interservice RPC latencies for three
	// distinct services with different latency distributions. These services are
	// differentiated via a "service" label.
	rpcDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "rpc_durations_microseconds",
			Help: "RPC latency distributions.",
		},
		[]string{"service"},
	)
)

func init() {
	// Register the summary with Prometheus's default registry.
	prometheus.MustRegister(rpcDurations)
}

func main() {
	flag.Parse()

	go func() {
		for {
			// Periodically record some sample latencies for the three services.
			rpcDurations.WithLabelValues("uniform").Observe(rand.Float64() * *uniformDomain)
			rpcDurations.WithLabelValues("normal").Observe((rand.NormFloat64() * *normDomain) + *normMean)
			rpcDurations.WithLabelValues("exponential").Observe(rand.ExpFloat64())

			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Expose the registered metrics via HTTP.
	http.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(*addr, nil)
}
