// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"encoding/json"

	dto "github.com/prometheus/client_model/go"
)

// A Metric is something that can be exposed via the registry framework.
type Metric interface {
	// Produce a JSON representation of the metric.
	json.Marshaler
	// Reset the parent metrics and delete all child metrics.
	ResetAll()
	// Produce a human-consumable representation of the metric.
	String() string
	// dumpChildren populates the child metrics of the given family.
	dumpChildren(*dto.MetricFamily)
	// samples calls fn for each of its samples. If fn returns an error, it will
	// terminate and return the error.
	samples(name string, fn sampleFn) error
}

// A sampleFn is provided to the samples method of metrics. Ownership of the
// labels map provided is assumed to belong to the caller, so implementations
// of this type should copy the labels if they intend to hold them.
type sampleFn func(name string, val float64, labels map[string]string) error
