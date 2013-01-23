// Copyright (c) 2013, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

package registry

var (
	// NilLabels is a nil set of labels merely for end-user convenience.
	NilLabels map[string]string = nil

	// A prefix to be used to namespace instrumentation flags from others.
	FlagNamespace = "telemetry."

	// The format of the exported data.  This will match this library's version,
	// which subscribes to the Semantic Versioning scheme.
	APIVersion = "0.0.1"

	ProtocolVersionHeader = "X-Prometheus-API-Version"

	baseLabelsKey = "baseLabels"
	docstringKey  = "docstring"
	metricKey     = "metric"
	nameLabel     = "name"
)
