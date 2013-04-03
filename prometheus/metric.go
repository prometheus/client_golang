// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

// A Metric is something that can be exposed via the registry framework.
type Metric interface {
	// Produce a JSON-consumable representation of the metric.
	AsMarshallable() map[string]interface{}
	// Reset the parent metrics and delete all child metrics.
	ResetAll()
	// Produce a human-consumable representation of the metric.
	String() string
}
