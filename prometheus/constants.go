// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package prometheus

var (
	// NilLabels is a nil set of labels merely for end-user convenience.
	NilLabels map[string]string = nil

	// The default http.Handler for exposing telemetric data over a web services
	// interface.
	DefaultHandler = DefaultRegistry.Handler()

	// This is the default registry with which Metric objects are associated.
	DefaultRegistry = NewRegistry()
)

const (
	// A prefix to be used to namespace instrumentation flags from others.
	FlagNamespace = "telemetry."

	// The format of the exported data.  This will match this library's version,
	// which subscribes to the Semantic Versioning scheme.
	APIVersion = "0.0.2"

	// The content type and schema information set on telemetry data responses.
	TelemetryContentType = `application/json; schema="prometheus/telemetry"; version=` + APIVersion
	// The content type and schema information set on telemetry data responses.
	DelimitedTelemetryContentType = `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="delimited"`

	// The customary web services endpoint on which telemetric data is exposed.
	ExpositionResource = "/metrics"

	baseLabelsKey = "baseLabels"
	docstringKey  = "docstring"
	metricKey     = "metric"

	counterTypeValue   = "counter"
	floatBitCount      = 64
	floatFormat        = 'f'
	floatPrecision     = 6
	gaugeTypeValue     = "gauge"
	histogramTypeValue = "histogram"
	typeKey            = "type"
	valueKey           = "value"
	labelsKey          = "labels"
)
