// Copyright 2026 The Prometheus Authors
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

// Package main demonstrates how to configure and expose Prometheus native histograms,
// including default usage, fine-tuning knobs, and dual-mode classic compatibility.
//
// Native histograms introduce exponential sparse bucketing that adapts to observed
// value distributions dynamically without requiring predefined bucket boundaries.
//
// Native histograms require Prometheus v2.40+ with the --enable-feature=native-histograms
// flag enabled, and are enabled by default in Prometheus v3. Scrapers must negotiate
// the PrometheusProto format:
//
//	scrape_configs:
//	  - job_name: 'histogram_example'
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

// histogramMetrics holds the sample histograms demonstrating different configuration modes.
type histogramMetrics struct {
	defaultHistogram   prometheus.Histogram
	fineTunedHistogram prometheus.Histogram
	dualModeHistogram  prometheus.Histogram
}

func newHistogramMetrics() *histogramMetrics {
	// 1. Simple default native histogram.
	// Setting NativeHistogramBucketFactor to a value > 1 enables sparse native buckets.
	// A factor of 1.1 limits the growth factor between adjacent buckets to at most 10%
	// (2^(2^-3) ≈ 1.0905, dividing each power of two into 8 buckets).
	// This provides high accuracy with balanced cardinality for typical latency metrics.
	defaultHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                        "http_request_duration_default_seconds",
		Help:                        "HTTP request latency using default native histogram settings.",
		NativeHistogramBucketFactor: 1.1,
	})

	// 2. Fine-tuned native histogram with resource controls and threshold tuning.
	// This configuration demonstrates tuning knobs for workloads spanning wide ranges
	// or subject to external inputs where bounded resource utilization is required:
	//
	// - NativeHistogramBucketFactor: Controls bucket resolution. Higher values (e.g. 1.2 or 1.5)
	//   produce wider buckets, reducing memory consumption and scrape payload size.
	// - NativeHistogramZeroThreshold: Sets the absolute threshold below which observations
	//   are directed to the zero bucket instead of allocating sparse buckets near zero.
	// - NativeHistogramMaxBucketNumber: Enforces an upper bound on populated sparse buckets.
	//   Without this limit, unbounded values could consume arbitrary memory or cause excessive
	//   scrape sizes. When exceeded, the histogram widens buckets or broadens the zero bucket.
	// - NativeHistogramMinResetDuration: Defines the cooldown interval before resetting
	//   bucket structures after exceeding NativeHistogramMaxBucketNumber. This prevents
	//   reset oscillation during temporary latency spikes while allowing eventual memory recovery.
	// - NativeHistogramMaxZeroThreshold: Sets an upper limit on zero bucket expansion when
	//   reducing bucket counts.
	fineTunedHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                            "http_request_duration_finetuned_seconds",
		Help:                            "HTTP request latency using fine-tuned native histogram settings.",
		NativeHistogramBucketFactor:     1.2,
		NativeHistogramZeroThreshold:    0.001,
		NativeHistogramMaxZeroThreshold: 0.01,
		NativeHistogramMaxBucketNumber:  100,
		NativeHistogramMinResetDuration: 15 * time.Minute,
	})

	// 3. Dual-mode histogram combining classic fixed buckets and native sparse buckets.
	// Defining Buckets together with NativeHistogramBucketFactor enables migration.
	// Scrapers requesting text formats receive classic cumulative buckets, while
	// scrapers negotiating PrometheusProto receive high-resolution native histograms.
	dualModeHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                        "http_request_duration_dualmode_seconds",
		Help:                        "HTTP request latency with both classic and native histogram bucketing.",
		Buckets:                     prometheus.DefBuckets,
		NativeHistogramBucketFactor: 1.1,
	})

	return &histogramMetrics{
		defaultHistogram:   defaultHistogram,
		fineTunedHistogram: fineTunedHistogram,
		dualModeHistogram:  dualModeHistogram,
	}
}

func setupRegistry(m *histogramMetrics) *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.defaultHistogram,
		m.fineTunedHistogram,
		m.dualModeHistogram,
	)
	return reg
}

// observeSample records a simulated observation across all sample histograms.
func (m *histogramMetrics) observeSample(latency float64) {
	if latency >= 0 {
		m.defaultHistogram.Observe(latency)
		m.fineTunedHistogram.Observe(latency)
		m.dualModeHistogram.Observe(latency)
	}
}

func main() {
	flag.Parse()

	metrics := newHistogramMetrics()
	reg := setupRegistry(metrics)

	// Simulate background request traffic with varying latency distributions.
	go func() {
		for {
			// Normal distribution around 150ms with occasional zero or outlier values.
			sample := rand.NormFloat64()*0.04 + 0.15
			if sample < 0 {
				sample = 0
			}
			metrics.observeSample(sample)
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// Expose metrics endpoint.
	// Testing native histogram protobuf output:
	//   curl -H 'Accept: application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited' localhost:8080/metrics
	//
	// Testing text output:
	//   curl -H 'Accept: application/openmetrics-text' localhost:8080/metrics
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	log.Printf("Listening for requests on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
