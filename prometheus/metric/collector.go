// Copyright 2015 The Prometheus Authors
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

package metric

// Collector is the interface implemented by anything that can be used by
// Prometheus to collect metrics. A Collector has to be registered for
// collection.
//
// The stock metrics provided in their respective packages (like Gauge, Counter,
// Summary, Histogram) are also Collectors (which only ever collect one metric,
// namely itself). An implementer of Collector may, however, collect multiple
// metrics in a coordinated fashion and/or create metrics on the fly. Examples
// for collectors already implemented in this library are the metric vectors
// (i.e. collection of multiple instances of the same Metric but with different
// label values) like gauge.Vec or summary.Vec, and the ExpvarCollector.
//
// Two Collectors are considered equal if their Describe methods yield the same
// set of descriptors.
type Collector interface {
	// Describe sends all descriptors required to describe the metrics
	// collected by this Collector to the provided channel and returns once
	// the last descriptor has been sent. The sent descriptors fulfill the
	// consistency and uniqueness requirements described in the Desc
	// documentation. This method idempotently sends the same descriptors
	// throughout the lifetime of the Collector. If a Collector encounters
	// an error while executing this method, it must send an invalid
	// descriptor (created with NewInvalidDesc) to signal the error to the
	// registry.
	Describe(chan<- Desc)
	// Collect is called by Prometheus when collecting metrics. The
	// implementation sends each collected metric via the provided channel
	// and returns once the last metric has been sent. Each sent metric must
	// be consistent with one of the descriptors returned by
	// Describe. Returned metrics that are described by the same descriptor
	// must differ in their variable label values. This method may be called
	// concurrently and must therefore be implemented in a concurrency safe
	// way. Blocking occurs at the expense of total performance of rendering
	// all registered metrics. Ideally, Collector implementations support
	// concurrent readers. If a Collector finds itself unable to collect a
	// metric, it can signal the error to the registry by sending a Metric
	// that will return the error in its Write method..
	Collect(chan<- Metric)
}
