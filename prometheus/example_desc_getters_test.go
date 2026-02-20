// Copyright 2025 The Prometheus Authors
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

package prometheus_test

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// ExampleDesc_getters demonstrates how to use the Desc getter methods
// to introspect metric metadata programmatically.
func ExampleDesc_getters() {
	// Create a descriptor directly
	desc := prometheus.NewDesc(
		"myapp_http_requests_total",
		"Total number of HTTP requests",
		[]string{"method", "status"},
		prometheus.Labels{
			"service": "api",
			"version": "1.0",
		},
	)

	// Use getter methods to access metadata
	fmt.Println("Metric Name:", desc.Name())
	fmt.Println("Help Text:", desc.Help())
	fmt.Println("Constant Labels:", desc.ConstLabels())
	fmt.Println("Variable Labels:", desc.VariableLabels())

	// Output:
	// Metric Name: myapp_http_requests_total
	// Help Text: Total number of HTTP requests
	// Constant Labels: map[service:api version:1.0]
	// Variable Labels: [method status]
}

// ExampleDesc_getters_documentation shows how to generate documentation
// from metric descriptors programmatically.
func ExampleDesc_getters_documentation() {
	// Create descriptors
	descriptors := []*prometheus.Desc{
		prometheus.NewDesc(
			"http_requests_total",
			"Total number of HTTP requests",
			nil,
			nil,
		),
		prometheus.NewDesc(
			"http_request_duration_seconds",
			"HTTP request duration in seconds",
			[]string{"method", "path"},
			nil,
		),
	}

	// Generate documentation
	fmt.Println("# Available Metrics")
	fmt.Println()
	for _, desc := range descriptors {
		fmt.Printf("## %s\n", desc.Name())
		fmt.Printf("**Help:** %s\n", desc.Help())

		if labels := desc.VariableLabels(); len(labels) > 0 {
			fmt.Printf("**Labels:** %v\n", labels)
		}

		if constLabels := desc.ConstLabels(); len(constLabels) > 0 {
			fmt.Printf("**Constant Labels:** %v\n", constLabels)
		}
		fmt.Println()
	}

	// Output:
	// # Available Metrics
	//
	// ## http_requests_total
	// **Help:** Total number of HTTP requests
	//
	// ## http_request_duration_seconds
	// **Help:** HTTP request duration in seconds
	// **Labels:** [method path]
}
