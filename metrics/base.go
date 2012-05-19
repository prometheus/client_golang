// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// base.go provides fundamental interface expectations for the various metrics.

package metrics

type Metric interface {
	// Produce a human-consumable representation of the metric.
	Humanize() string
	// Produce a JSON-consumable representation of the metric.
	// TODO(mtp):
	Marshallable() map[string]interface{}
}
