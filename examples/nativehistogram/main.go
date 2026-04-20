// Copyright The Prometheus Authors
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

// A simple example of how to use native histograms.
//
// Native histograms provide automatic bucketing with exponentially spaced
// bucket boundaries. Unlike classic histograms, there is no need to
// pre-define bucket boundaries -- the resolution adapts to the observed
// data distribution.
//
// Native histograms require Prometheus v2.40+ with the
// --enable-feature=native-histograms flag. Starting with Prometheus v3,
// native histograms are enabled by default.
//
// To scrape native histograms, configure Prometheus with:
//
//	scrape_configs:
//	  - job_name: 'example'
//	    scrape_protocols: ['PrometheusProto']
//	    static_configs:
//	      - targets: ['localhost:8080']
package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")

func main() {
	flag.Parse()

	// Native histogram: set NativeHistogramBucketFactor to a value > 1.
	// The factor controls bucket resolution. A value of 1.1 means each
	// bucket boundary is at most 10% wider than the previous one, which
	// is a good default for most use cases.
	requestDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                        "http_request_duration_seconds",
		Help:                        "HTTP request latency distribution.",
		NativeHistogramBucketFactor: 1.1,
	})

	// A native histogram tracking response sizes in bytes. Using a higher
	// bucket factor (1.5) produces fewer buckets, which reduces cost at
	// the expense of some precision.
	responseSize := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                            "http_response_size_bytes",
		Help:                            "HTTP response size distribution.",
		NativeHistogramBucketFactor:     1.5,
		NativeHistogramZeroThreshold:    1,
		NativeHistogramMaxBucketNumber:  100,
		NativeHistogramMinResetDuration: 15 * time.Minute,
	})

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		requestDuration,
		responseSize,
	)

	// Simulate request latencies and response sizes in the background.
	go func() {
		for {
			// Latency: normal distribution around 200ms.
			latency := rand.NormFloat64()*0.05 + 0.2
			if latency > 0 {
				requestDuration.Observe(latency)
			}

			// Response size: log-normal distribution.
			size := rand.ExpFloat64() * 1024
			responseSize.Observe(size)

			time.Sleep(100 * time.Millisecond)
		}
	}()

	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	// To test native histogram output (binary protobuf):
	//   curl -H 'Accept: application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited' localhost:8080/metrics
	//
	// For a human-readable response (native histogram buckets are not fully
	// represented in text formats):
	//   curl -H 'Accept: application/openmetrics-text' localhost:8080/metrics
	log.Fatal(http.ListenAndServe(*addr, nil))
}
