/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style license that can be found in
the LICENSE file.
*/

package registry

import (
	"encoding/base64"
	"encoding/json"
	"github.com/matttproud/golang_instrumentation/maths"
	"github.com/matttproud/golang_instrumentation/metrics"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	jsonContentType = "application/json"
	contentType     = "Content-Type"
	jsonSuffix      = ".json"
	authorization   = "Authorization"
)

/*
Boilerplate metrics about the metrics reporting subservice.  These are only
exposed if the DefaultRegistry's exporter is hooked into the HTTP request
handler.
*/

var requestCount *metrics.CounterMetric = &metrics.CounterMetric{}
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

/*
This callback accumulates the microsecond duration of the reporting framework's
overhead such that it can be reported.
*/
var requestLatencyAccumulator metrics.CompletionCallback = func(duration time.Duration) {
	microseconds := float64(int64(duration) / 1E3)

	requestLatencyLogarithmicAccumulating.Add(microseconds)
	requestLatencyEqualAccumulating.Add(microseconds)
	requestLatencyLogarithmicTallying.Add(microseconds)
	requestLatencyEqualTallying.Add(microseconds)
}

/*
Registry is, as the name implies, a registrar where metrics are listed.

In most situations, using DefaultRegistry is sufficient versus creating one's
own.
*/
type Registry struct {
	mutex        sync.RWMutex
	NameToMetric map[string]metrics.Metric
}

/*
This builds a new metric registry.  It is not needed in the majority of
cases.
*/
func NewRegistry() *Registry {
	return &Registry{
		NameToMetric: make(map[string]metrics.Metric),
	}
}

/*
This is the default registry with which Metric objects are associated.  It
is primarily a read-only object after server instantiation.
*/
var DefaultRegistry = NewRegistry()

/*
Associate a Metric with the DefaultRegistry.
*/
func Register(name, unusedDocstring string, unusedBaseLabels map[string]string, metric metrics.Metric) {
	DefaultRegistry.Register(name, unusedDocstring, unusedBaseLabels, metric)
}

/*
Register a metric with a given name.  Name should be globally unique.
*/
func (r *Registry) Register(name, unusedDocstring string, unusedBaseLabels map[string]string, metric metrics.Metric) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, present := r.NameToMetric[name]; !present {
		r.NameToMetric[name] = metric
		log.Printf("Registered %s.\n", name)
	} else {
		log.Printf("Attempted to register duplicate %s metric.\n", name)
	}
}

func (register *Registry) YieldBasicAuthExporter(username, password string) http.HandlerFunc {
	exporter := register.YieldExporter()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticated := false

		if auth := r.Header.Get(authorization); auth != "" {
			base64Encoded := strings.SplitAfter(auth, " ")[1]
			decoded, err := base64.URLEncoding.DecodeString(base64Encoded)
			if err == nil {
				usernamePassword := strings.Split(string(decoded), ":")
				if usernamePassword[0] == username && usernamePassword[1] == password {
					authenticated = true
				}
			}
		}

		if authenticated {
			exporter.ServeHTTP(w, r)
		} else {
			w.Header().Add("WWW-Authenticate", "Basic")
			http.Error(w, "access forbidden", 401)
		}
	})
}

/*
Create a http.HandlerFunc that is tied to r Registry such that requests
against it generate a representation of the housed metrics.
*/
func (registry *Registry) YieldExporter() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var instrumentable metrics.InstrumentableCall = func() {
			requestCount.Increment()
			url := r.URL

			if strings.HasSuffix(url.Path, jsonSuffix) {
				w.Header().Set(contentType, jsonContentType)
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
	nilBaseLabels := make(map[string]string)

	DefaultRegistry.Register("requests_metrics_total", "A counter of the total requests made against the telemetry system.", nilBaseLabels, requestCount)
	DefaultRegistry.Register("requests_metrics_latency_logarithmic_accumulating_microseconds", "A histogram of the response latency for requests made against the telemetry system.", nilBaseLabels, requestLatencyLogarithmicAccumulating)
	DefaultRegistry.Register("requests_metrics_latency_equal_accumulating_microseconds", "A histogram of the response latency for requests made against the telemetry system.", nilBaseLabels, requestLatencyEqualAccumulating)
	DefaultRegistry.Register("requests_metrics_latency_logarithmic_tallying_microseconds", "A histogram of the response latency for requests made against the telemetry system.", nilBaseLabels, requestLatencyLogarithmicTallying)
	DefaultRegistry.Register("request_metrics_latency_equal_tallying_microseconds", "A histogram of the response latency for requests made against the telemetry system.", nilBaseLabels, requestLatencyEqualTallying)
}
