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

// This example demonstrates how to use Registry.Descriptors() to introspect
// all registered metrics programmatically. This is useful for generating
// documentation, validating metric configurations, or implementing custom
// metric discovery systems.
package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var mode = flag.String("mode", "list", "Mode: list, docs, or filter")
var filter = flag.String("filter", "http", "Prefix to filter metrics by (only used in 'filter' mode)")

func main() {
	flag.Parse()

	// Create a custom registry
	reg := prometheus.NewRegistry()

	// Register various metrics to demonstrate the feature
	registerExampleMetrics(reg)

	// Get all registered descriptors
	descs := reg.Descriptors()

	// Sort by name for consistent output
	sort.Slice(descs, func(i, j int) bool {
		return descs[i].Name() < descs[j].Name()
	})

	switch *mode {
	case "list":
		listMetrics(descs)
	case "docs":
		generateDocumentation(descs)
	case "filter":
		filterMetrics(descs, *filter)
	default:
		fmt.Printf("Unknown mode: %s\n", *mode)
		fmt.Println("Available modes: list, docs, filter")
	}
}

func registerExampleMetrics(reg *prometheus.Registry) {
	// HTTP metrics
	httpRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "status"},
	)
	reg.MustRegister(httpRequests)

	httpDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request duration in seconds",
		},
		[]string{"method", "path"},
	)
	reg.MustRegister(httpDuration)

	httpErrors := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "Total number of HTTP errors",
		},
	)
	reg.MustRegister(httpErrors)

	// Database metrics
	dbConnections := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "database_connections",
			Help:        "Number of active database connections",
			ConstLabels: prometheus.Labels{"service": "api", "version": "1.0"},
		},
		[]string{"state"},
	)
	reg.MustRegister(dbConnections)

	dbQueryDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "database_query_duration_seconds",
			Help: "Database query duration in seconds",
		},
	)
	reg.MustRegister(dbQueryDuration)

	// System metrics
	cpuUsage := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "cpu_usage_percent",
			Help: "Current CPU usage percentage",
		},
	)
	reg.MustRegister(cpuUsage)

	memoryUsage := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
	)
	reg.MustRegister(memoryUsage)
}

func listMetrics(descs []*prometheus.Desc) {
	fmt.Println("=== Registered Metrics ===")
	fmt.Println()

	for i, desc := range descs {
		fmt.Printf("%d. %s\n", i+1, desc.Name())
		fmt.Printf("   Help: %s\n", desc.Help())

		if labels := desc.VariableLabels(); len(labels) > 0 {
			fmt.Printf("   Variable Labels: %v\n", labels)
		}

		if constLabels := desc.ConstLabels(); len(constLabels) > 0 {
			fmt.Printf("   Constant Labels: %v\n", constLabels)
		}

		fmt.Println()
	}

	fmt.Printf("Total metrics: %d\n", len(descs))
}

func generateDocumentation(descs []*prometheus.Desc) {
	fmt.Println("# Metrics Documentation")
	fmt.Println()
	fmt.Println("This documentation was automatically generated from the registry.")
	fmt.Println()

	// Group metrics by prefix
	groups := make(map[string][]*prometheus.Desc)
	for _, desc := range descs {
		parts := strings.SplitN(desc.Name(), "_", 2)
		prefix := parts[0]
		groups[prefix] = append(groups[prefix], desc)
	}

	// Get sorted group names
	var groupNames []string
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	// Print each group
	for _, groupName := range groupNames {
		fmt.Printf("## %s Metrics\n\n", strings.Title(groupName))

		for _, desc := range groups[groupName] {
			fmt.Printf("### `%s`\n\n", desc.Name())
			fmt.Printf("%s\n\n", desc.Help())

			if labels := desc.VariableLabels(); len(labels) > 0 {
				fmt.Printf("**Labels:**\n")
				for _, label := range labels {
					fmt.Printf("- `%s`\n", label)
				}
				fmt.Println()
			}

			if constLabels := desc.ConstLabels(); len(constLabels) > 0 {
				fmt.Printf("**Constant Labels:**\n")
				var keys []string
				for k := range constLabels {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Printf("- `%s`: `%s`\n", k, constLabels[k])
				}
				fmt.Println()
			}
		}
	}
}

func filterMetrics(descs []*prometheus.Desc, prefix string) {
	fmt.Printf("=== Metrics matching prefix '%s' ===\n\n", prefix)

	var matches []*prometheus.Desc
	for _, desc := range descs {
		if strings.HasPrefix(desc.Name(), prefix) {
			matches = append(matches, desc)
		}
	}

	if len(matches) == 0 {
		fmt.Printf("No metrics found with prefix '%s'\n", prefix)
		return
	}

	for _, desc := range matches {
		fmt.Printf("• %s\n", desc.Name())
		fmt.Printf("  %s\n", desc.Help())

		if labels := desc.VariableLabels(); len(labels) > 0 {
			fmt.Printf("  Labels: %v\n", labels)
		}

		fmt.Println()
	}

	fmt.Printf("Found %d matching metric(s)\n", len(matches))
}
