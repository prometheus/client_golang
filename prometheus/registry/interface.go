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

// Package registry provides the interface of the metrics registry and means to
// instantiate implementations thereof. It also provides the so-called default
// registry, a pre-configured instantiation of a registry that should be
// sufficient for most use-cases.
package registry

import (
	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus/metric"
)

// Registry is the interface for the metrics registry.
type Registry interface {
	// Register registers a new Collector to be included in metrics
	// collection. It returns an error if the descriptors provided by the
	// Collector are invalid or if they - in combination with descriptors of
	// already registered Collectors - do not fulfill the consistency and
	// uniqueness criteria described in the documentation of metric.Desc.
	//
	// If the provided Collector is equal to a Collector already registered
	// (which includes the case of re-registering the same Collector), the
	// returned error is an instance of AlreadyRegisteredError, which
	// contains the previously registered Collector.
	//
	// It is in general not safe to register the same Collector multiple
	// times concurrently.
	Register(metric.Collector) error
	// Unregister unregisters the Collector that equals the Collector passed
	// in as an argument. The function returns whether a Collector was
	// unregistered.
	Unregister(metric.Collector) bool
	// Collect returns a channel that yields MetricFamily protobufs
	// (collected from registered collectors) together with applicable
	// errors. The metric family pointer returned with an error could be nil
	// or point to a (presumably incomplete) metric family. Once all
	// MetricFamilies have been read, the channel is closed. To not leak
	// resources, the channel must always be read until closed, even if one
	// ore more errors have been returned. If names is nil or empty, all
	// MetricFamilies are returned. Otherwise, only MetricFamilies with a
	// name contained in names are returned. Implementations should aim for
	// lexicographical sorting of MetricFamilies if the resource cost of
	// sorting is not prohibitive.
	Collect(names map[string]struct{}) <-chan struct {
		*dto.MetricFamily
		error
	}
}

// Opts are options for the instantiation of new registries. The zero value of
// Opts is a safe default.
type Opts struct {
	// If true, metrics are checked for consistency during collection. The
	// check has a performance overhead and is not necessary with
	// well-behaved collectors. It can be helpful to enable the check while
	// working with custom Collectors whose correctness is not well
	// established yet or where inconsistent collection might happen by
	// design.
	CollectCheckEnabled bool
	// If true, the channel returned by the Collect method will never yield
	// an error (so that no error handling has to be implemented when
	// receiving from the channel). Instead, the program will panic. This
	// behavior is useful in programs where collect errors cannot (or must
	// not) happen.
	PanicOnCollectError bool
	// The MetricFamilyInjectionHook is a function that is called whenever
	// metrics are collected. The MetricFamily protobufs returned by the
	// hook function are merged with the metrics collected in the usual way.
	//
	// This is a way to directly inject MetricFamily protobufs managed and
	// owned by the caller. The caller has full responsibility. As no
	// registration of the injected metrics has happened, there was no check
	// at registration-time. If CollectCheckEnabled is false, only very
	// limited sanity checks are performed on the returned protobufs.
	//
	// Sorting concerns: The caller is responsible for sorting the label
	// pairs in each metric. However, the order of metrics will be sorted by
	// the registry as it is required anyway after merging with the metric
	// families collected conventionally.
	//
	// The function must be callable at any time and concurrently.
	MetricFamilyInjectionHook func() []*dto.MetricFamily
}
