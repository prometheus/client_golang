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

import dto "github.com/prometheus/client_model/go"

// A Metric models a single sample value with its meta data being exported to
// Prometheus. Implementers of Metric in this package include Gauge, Counter,
// Untyped, and Summary. Users can implement their own Metric types, but that
// should be rarely needed.
type Metric interface {
	// Desc returns the descriptor for the Metric. This method idempotently
	// returns the same descriptor throughout the lifetime of the Metric. A
	// Metric unable to describe itself must return an invalid descriptor
	// (created with NewInvalidDesc).
	Desc() Desc
	// Write encodes the Metric into a "Metric" Protocol Buffer data
	// transmission object.
	//
	// Implementers of custom Metric types must observe concurrency safety
	// as reads of this metric may occur at any time, and any blocking
	// occurs at the expense of total performance of rendering all
	// registered metrics. Ideally Metric implementations should support
	// concurrent readers.
	//
	// The caller may minimize memory allocations by providing a
	// pre-existing reset dto.Metric pointer. The caller may recycle the
	// dto.Metric proto message, so Metric implementations should just
	// populate the provided dto.Metric and then should not keep any
	// reference to it.
	//
	// While populating dto.Metric, labels must be sorted lexicographically.
	// (Implementers may find LabelPairSorter useful for that.)
	Write(*dto.Metric) error
}
