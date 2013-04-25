// Copyright (c) 2013, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package registry

var (
	// NilLabels is a nil set of labels merely for end-user convenience.
	NilLabels map[string]string = nil

	// The default http.Handler for exposing telemetric data over a web services
	// interface.
	DefaultHandler = DefaultRegistry.Handler()

	// This is the default registry with which Metric objects are associated.  It
	// is primarily a read-only object after server instantiation.
	DefaultRegistry = NewRegistry()
)

const (
	// A prefix to be used to namespace instrumentation flags from others.
	FlagNamespace = "telemetry."

	// The format of the exported data.  This will match this library's version,
	// which subscribes to the Semantic Versioning scheme.
	APIVersion = "0.0.1"

	// When reporting telemetric data over the HTTP web services interface, a web
	// services interface shall include this Content-Type along with APIVersion as its
	// value.
	ProtocolContentType = "Content-Type: application/json; protocol=prometheus_telemetry; version=" + APIVersion

	// The customary web services endpoint on which telemetric data is exposed.
	ExpositionResource = "/metrics.json"

	baseLabelsKey = "baseLabels"
	docstringKey  = "docstring"
	metricKey     = "metric"
	nameLabel     = "name"
)
