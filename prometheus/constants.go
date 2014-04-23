// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package prometheus

var (
	// NilLabels is a nil set of labels merely for end-user convenience.
	NilLabels map[string]string

	// DefaultHandler is the default http.Handler for exposing telemetric
	// data over a web services interface.
	DefaultHandler = DefaultRegistry.Handler()

	// DefaultRegistry with which Metric objects are associated.
	DefaultRegistry = NewRegistry()
)

const (
	// FlagNamespace is a prefix to be used to namespace instrumentation
	// flags from others.
	FlagNamespace = "telemetry."

	// APIVersion is the version of the format of the exported data.  This
	// will match this library's version, which subscribes to the Semantic
	// Versioning scheme.
	APIVersion = "0.0.4"

	// JSONAPIVersion is the version of the JSON export format.
	JSONAPIVersion = "0.0.2"

	// DelimitedTelemetryContentType is the content type set on telemetry data responses in delimited protobuf format.
	DelimitedTelemetryContentType = `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="delimited"`
	// TextTelemetryContentType is the content type set on telemetry data responses in text format.
	TextTelemetryContentType = `text/plain; version=` + APIVersion
	// ProtoTextTelemetryContentType is the content type set on telemetry data responses in protobuf text format.
	// (Only used for debugging.)
	ProtoTextTelemetryContentType = `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="text"`
	// ProtoCompactTextTelemetryContentType is the content type set on telemetry data responses in protobuf compact text format.
	// (Only used for debugging.)
	ProtoCompactTextTelemetryContentType = `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="compact-text"`
	// JSONTelemetryContentType is the content type set on telemetry data
	// responses formatted as JSON.
	JSONTelemetryContentType = `application/json; schema="prometheus/telemetry"; version=` + JSONAPIVersion

	// ExpositionResource is the customary web services endpoint on which
	// telemetric data is exposed.
	ExpositionResource = "/metrics"

	baseLabelsKey = "baseLabels"
	docstringKey  = "docstring"
	metricKey     = "metric"

	counterTypeValue   = "counter"
	floatBitCount      = 64
	floatFormat        = 'f'
	floatPrecision     = 6
	gaugeTypeValue     = "gauge"
	untypedTypeValue   = "untyped"
	histogramTypeValue = "histogram"
	typeKey            = "type"
	valueKey           = "value"
	labelsKey          = "labels"
)

var blankLabelsSingleton = map[string]string{}
