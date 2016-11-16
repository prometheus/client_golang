// Copyright 2014 The Prometheus Authors
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

package prometheus_test

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// For periodic or batch processes that run much less frequently than
	// the Prometheus scrape, it makes sense to use a Gauge to observe the
	// duration.
	// See https://prometheus.io/docs/practices/instrumentation/#batch-jobs
	batchDuration = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "example_batch_duration_seconds",
		Help: "Duration of the last batch run.",
	})
)

func performBatch() {
	// The Set method of the Gauge is used to observe the duration.
	timer := prometheus.NewTimer(prometheus.ObserverFunc(batchDuration.Set))
	defer timer.ObserveDuration()

	// Actually perform the batch of work.
	// ...
}

func ExampleTimer_batch() {
	// 10m is much longer than the usual scrape interval of Prometheus.
	c := time.Tick(10 * time.Minute)
	for _ = range c {
		performBatch()
	}
}
