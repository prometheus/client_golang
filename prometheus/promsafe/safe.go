// Copyright 2024 The Prometheus Authors
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

// Package promsafe provides a layer of type-safety for label management in Prometheus metrics.
//
// Promsafe introduces type-safe labels, ensuring that the labels used with
// Prometheus metrics are explicitly defined and validated at compile-time. This
// eliminates common runtime errors caused by mislabeling, such as typos or
// incorrect label orders.
//
// The following example demonstrates how to create and use a CounterVec with
// type-safe labels (compared to how it's done in a regular way):
//
//	package main
//
//	import (
//		"strconv"
//
//		"github.com/prometheus/client_golang/prometheus"
//		"github.com/prometheus/client_golang/prometheus/promsafe"
//	)
//
//	// Original unsafe way (no type safety)
//
//	func originalUnsafeWay() {
//		counterVec := prometheus.NewCounterVec(
//			prometheus.CounterOpts{
//				Name: "http_requests_total",
//				Help: "Total number of HTTP requests by status code and method.",
//			},
//			[]string{"code", "method"}, // Labels defined as raw strings
//		)
//
//		// No compile-time checks; label order and types must be correct
//		// You have to know which and how many labels are expected (in proper order)
//		counterVec.WithLabelValues("200", "GET").Inc()
//
//		// or you can use map, that is even more fragile
//		counterVec.With(prometheus.Labels{"code": "200", "method": "GET"}).Inc()
//	}
//
//	// Safe way (Quick implementation, reflect-based under-the-hood)
//
//	type Labels1 struct {
//		promsafe.StructLabelProvider
//		Code   int
//		Method string
//	}
//
//	func safeReflectWay() {
//		counterVec := promsafe.NewCounterVec[Labels1](prometheus.CounterOpts{
//			Name: "http_requests_total_reflection",
//			Help: "Total number of HTTP requests by status code and method (reflection-based).",
//		})
//
//		// Compile-time safe and readable; Will be converted into properly ordered list: "200", "GET"
//		counterVec.With(Labels1{Method: "GET", Code: 200}).Inc()
//	}
//
//	// Safe way with manual implementation (no reflection overhead, as fast as original)
//
//	type Labels2 struct {
//		promsafe.StructLabelProvider
//		Code   int
//		Method string
//	}
//
//	func (c Labels2) ToPrometheusLabels() prometheus.Labels {
//		return prometheus.Labels{
//			"code":   strconv.Itoa(c.Code), // Convert int to string
//			"method": c.Method,
//		}
//	}
//
//	func (c Labels2) ToLabelNames() []string {
//		return []string{"code", "method"}
//	}
//
//	func safeManualWay() {
//		counterVec := promsafe.NewCounterVec[Labels2](prometheus.CounterOpts{
//			Name: "http_requests_total_custom",
//			Help: "Total number of HTTP requests by status code and method (manual implementation).",
//		})
//		counterVec.With(Labels2{Code: 404, Method: "POST"}).Inc()
//	}
//
// Package promsafe also provides compatibility adapter for integration with Prometheus's
// `promauto` package, ensuring seamless adoption while preserving type-safety.
// Methods that cannot guarantee type safety, such as those using raw `[]string`
// label values, are explicitly deprecated and will raise runtime errors.
//
// A separate package allows conservative users to entirely ignore it. And
// whoever wants to use it will do so explicitly, with an opportunity to read
// this warning.
//
// Enjoy promsafe as it's safe!
package promsafe

import (
	"github.com/prometheus/client_golang/prometheus"
)

// NewCounterVec creates a new CounterVec with type-safe labels.
func NewCounterVec[T LabelsProviderMarker](opts prometheus.CounterOpts) *CounterVec[T] {
	emptyLabels := NewEmptyLabels[T]()
	inner := prometheus.NewCounterVec(opts, extractLabelNames(emptyLabels))

	return &CounterVec[T]{inner: inner}
}

// CounterVec is a wrapper around prometheus.CounterVec that allows type-safe labels.
type CounterVec[T LabelsProviderMarker] struct {
	inner *prometheus.CounterVec
}

// GetMetricWithLabelValues covers prometheus.CounterVec.GetMetricWithLabelValues
// Deprecated: Use GetMetricWith() instead. We can't provide a []string safe implementation in promsafe
func (c *CounterVec[T]) GetMetricWithLabelValues(_ ...string) (prometheus.Counter, error) {
	panic("There can't be a SAFE GetMetricWithLabelValues(). Use GetMetricWith() instead")
}

// GetMetricWith behaves like prometheus.CounterVec.GetMetricWith but with type-safe labels.
func (c *CounterVec[T]) GetMetricWith(labels T) (prometheus.Counter, error) {
	return c.inner.GetMetricWith(extractLabelsWithValues(labels))
}

// WithLabelValues covers like prometheus.CounterVec.WithLabelValues.
// Deprecated: Use With() instead. We can't provide a []string safe implementation in promsafe
func (c *CounterVec[T]) WithLabelValues(_ ...string) prometheus.Counter {
	panic("There can't be a SAFE WithLabelValues(). Use With() instead")
}

// With behaves like prometheus.CounterVec.With but with type-safe labels.
func (c *CounterVec[T]) With(labels T) prometheus.Counter {
	return c.inner.With(extractLabelsWithValues(labels))
}

// CurryWith behaves like prometheus.CounterVec.CurryWith but with type-safe labels.
// It still returns a CounterVec, but it's inner prometheus.CounterVec is curried.
func (c *CounterVec[T]) CurryWith(labels T) (*CounterVec[T], error) {
	curriedInner, err := c.inner.CurryWith(extractLabelsWithValues(labels))
	if err != nil {
		return nil, err
	}
	c.inner = curriedInner
	return c, nil
}

// MustCurryWith behaves like prometheus.CounterVec.MustCurryWith but with type-safe labels.
// It still returns a CounterVec, but it's inner prometheus.CounterVec is curried.
func (c *CounterVec[T]) MustCurryWith(labels T) *CounterVec[T] {
	c.inner = c.inner.MustCurryWith(extractLabelsWithValues(labels))
	return c
}

// Unsafe returns the underlying prometheus.CounterVec
// it's used to call any other method of prometheus.CounterVec that doesn't require type-safe labels
func (c *CounterVec[T]) Unsafe() *prometheus.CounterVec {
	return c.inner
}

// NewCounter simply creates a new prometheus.Counter.
// As it doesn't have any labels, it's already type-safe.
// We keep this method just for consistency and interface fulfillment.
func NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	return prometheus.NewCounter(opts)
}

// NewCounterFunc wraps a new prometheus.CounterFunc.
// As it doesn't have any labels, it's already type-safe.
// We keep this method just for consistency and interface fulfillment.
func NewCounterFunc(opts prometheus.CounterOpts, function func() float64) prometheus.CounterFunc {
	return prometheus.NewCounterFunc(opts, function)
}

// TODO: other methods (Gauge, Histogram, Summary, etc.)
