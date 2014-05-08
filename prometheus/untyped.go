// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import dto "github.com/prometheus/client_model/go"

// Untyped proxies an untyped scalar value.
type Untyped interface {
	Metric
	MetricsCollector

	// Set assigns the value of this Untyped metric to the proxied value.
	Set(float64, ...string)
	// Del deletes a given label set from this Untyped metric, indicating
	// whether the label set was deleted.
	Del(...string) bool
}

// NewUntyped emits a new Untyped metric from the provided descriptor.
// The descriptor's Type field is ignored and forcefully set to MetricType_UNTYPED.
func NewUntyped(desc *Desc) Untyped {
	desc.Type = dto.MetricType_UNTYPED
	return NewValueMetric(desc)
}
