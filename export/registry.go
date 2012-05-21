// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// registry.go provides a container for centralization exposition of metrics to
// their prospective consumers.

package export

import (
	"encoding/json"
	"github.com/matttproud/golang_instrumentation/maths"
	"github.com/matttproud/golang_instrumentation/metrics"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var requestCount *metrics.GaugeMetric = &metrics.GaugeMetric{}
var requestLatencyLogarithmicBuckets []float64 = metrics.LogarithmicSizedBucketsFor(0, 1000)
var requestLatencyEqualBuckets []float64 = metrics.EquallySizedBucketsFor(0, 1000, 10)
var requestLatencyLogarithmicAccumulating *metrics.Histogram = metrics.CreateHistogram(&metrics.HistogramSpecification{
	Starts:                requestLatencyLogarithmicBuckets,
	BucketMaker:           metrics.AccumulatingBucketBuilder(metrics.EvictAndReplaceWith(50, maths.Average), 1000),
	ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.9, 0.99},
})
var requestLatencyEqualAccumulating *metrics.Histogram = metrics.CreateHistogram(&metrics.HistogramSpecification{
	Starts:                requestLatencyEqualBuckets,
	BucketMaker:           metrics.AccumulatingBucketBuilder(metrics.EvictAndReplaceWith(50, maths.Average), 1000),
	ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.9, 0.99},
})
var requestLatencyLogarithmicTallying *metrics.Histogram = metrics.CreateHistogram(&metrics.HistogramSpecification{
	Starts:                requestLatencyLogarithmicBuckets,
	BucketMaker:           metrics.TallyingBucketBuilder,
	ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.9, 0.99},
})
var requestLatencyEqualTallying *metrics.Histogram = metrics.CreateHistogram(&metrics.HistogramSpecification{
	Starts:                requestLatencyEqualBuckets,
	BucketMaker:           metrics.TallyingBucketBuilder,
	ReportablePercentiles: []float64{0.01, 0.05, 0.5, 0.9, 0.99},
})

var requestLatencyAccumulator metrics.CompletionCallback = func(duration time.Duration) {
	micros := float64(int64(duration) / 1E3)

	requestLatencyLogarithmicAccumulating.Add(micros)
	requestLatencyEqualAccumulating.Add(micros)
	requestLatencyLogarithmicTallying.Add(micros)
	requestLatencyEqualTallying.Add(micros)
}

// Registry is, as the name implies, a registrar where metrics are listed.
//
// In most situations, using DefaultRegistry is sufficient versus creating
// one's own.
type Registry struct {
	mutex        sync.RWMutex
	NameToMetric map[string]metrics.Metric
}

// This builds a new metric registry.  It is not needed in the majority of
// cases.
func NewRegistry() *Registry {
	return &Registry{
		NameToMetric: make(map[string]metrics.Metric),
	}
}

// This is the default registry with which Metric objects are associated.  It
// is primarily a read-only object after server instantiation.
var DefaultRegistry = NewRegistry()

// Associate a Metric with the DefaultRegistry.
func Register(name string, metric metrics.Metric) {
	DefaultRegistry.Register(name, metric)
}

// Register a metric with a given name.  Name should be globally unique.
func (r *Registry) Register(name string, metric metrics.Metric) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, present := r.NameToMetric[name]; !present {
		r.NameToMetric[name] = metric
		log.Printf("Registered %s.\n", name)
	} else {
		log.Printf("Attempted to register duplicate %s metric.\n", name)
	}
}

// Create a http.HandlerFunc that is tied to r Registry such that requests
// against it generate a representation of the housed metrics.
func (registry *Registry) YieldExporter() *http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var instrumentable metrics.InstrumentableCall = func() {
			requestCount.Increment()
			url := r.URL

			if strings.HasSuffix(url.Path, ".json") {
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
				composite := make(map[string]interface{}, len(registry.NameToMetric))
				for name, metric := range registry.NameToMetric {
					composite[name] = metric.Marshallable()
				}

				data, _ := json.Marshal(composite)

				w.Write(data)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}

		}
		metrics.InstrumentCall(instrumentable, requestLatencyAccumulator)
	}
}

func init() {
	DefaultRegistry.Register("requests_total", requestCount)
	DefaultRegistry.Register("request_latency_logarithmic_accumulating_microseconds", requestLatencyLogarithmicAccumulating)
	DefaultRegistry.Register("request_latency_equal_accumulating_microseconds", requestLatencyEqualAccumulating)
	DefaultRegistry.Register("request_latency_logarithmic_tallying_microseconds", requestLatencyLogarithmicTallying)
	DefaultRegistry.Register("request_latency_equal_tallying_microseconds", requestLatencyEqualTallying)
}
