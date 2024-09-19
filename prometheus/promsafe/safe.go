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

// Package promsafe provides safe labeling - strongly typed labels in prometheus metrics.
// Enjoy promsafe as you wish!
package promsafe

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

//
// promsafe configuration: promauto-compatibility, etc
//

// factory stands for a global promauto.Factory to be used (if any)
var factory *promauto.Factory

// SetupGlobalPromauto sets a global promauto.Factory to be used for all promsafe metrics.
// This means that each promsafe.New* call will use this promauto.Factory.
func SetupGlobalPromauto(factoryArg ...promauto.Factory) {
	if len(factoryArg) == 0 {
		f := promauto.With(prometheus.DefaultRegisterer)
		factory = &f
	} else {
		f := factoryArg[0]
		factory = &f
	}
}

// promsafeTag is the tag name used for promsafe labels inside structs.
// The tag is optional, as if not present, field is used with snake_cased FieldName.
// It's useful to use a tag when you want to override the default naming or exclude a field from the metric.
var promsafeTag = "promsafe"

// SetPromsafeTag sets the tag name used for promsafe labels inside structs.
func SetPromsafeTag(tag string) {
	promsafeTag = tag
}

// labelsProviderMarker is a marker interface for enforcing type-safety of StructLabelProvider.
type labelsProviderMarker interface {
	labelsProviderMarker()
}

// StructLabelProvider should be embedded in any struct that serves as a label provider.
type StructLabelProvider struct{}

var _ labelsProviderMarker = (*StructLabelProvider)(nil)

func (s StructLabelProvider) labelsProviderMarker() {
	panic("labelsProviderMarker interface method should never be called")
}

// newEmptyLabels creates a new empty labels instance of type T
// It's a bit tricky as we want to support both structs and pointers to structs
// e.g. &MyLabels{StructLabelProvider} or MyLabels{StructLabelProvider}
func newEmptyLabels[T labelsProviderMarker]() T {
	var emptyLabels T

	// Let's Support both Structs or Pointer to Structs given as T
	val := reflect.ValueOf(&emptyLabels).Elem()
	if val.Kind() == reflect.Ptr {
		val.Set(reflect.New(val.Type().Elem()))
	}

	return emptyLabels
}

// NewCounterVecT creates a new CounterVecT with type-safe labels.
func NewCounterVecT[T labelsProviderMarker](opts prometheus.CounterOpts) *CounterVecT[T] {
	emptyLabels := newEmptyLabels[T]()

	var inner *prometheus.CounterVec
	if factory != nil {
		inner = factory.NewCounterVec(opts, extractLabelNames(emptyLabels))
	} else {
		inner = prometheus.NewCounterVec(opts, extractLabelNames(emptyLabels))
	}

	return &CounterVecT[T]{inner: inner}
}

// CounterVecT is a wrapper around prometheus.CounterVec that allows type-safe labels.
type CounterVecT[T labelsProviderMarker] struct {
	inner *prometheus.CounterVec
}

// GetMetricWithLabelValues behaves like prometheus.CounterVec.GetMetricWithLabelValues but with type-safe labels.
func (c *CounterVecT[T]) GetMetricWithLabelValues(labels T) (prometheus.Counter, error) {
	return c.inner.GetMetricWithLabelValues(extractLabelValues(labels)...)
}

// GetMetricWith behaves like prometheus.CounterVec.GetMetricWith but with type-safe labels.
func (c *CounterVecT[T]) GetMetricWith(labels T) (prometheus.Counter, error) {
	return c.inner.GetMetricWith(extractLabelsWithValues(labels))
}

// WithLabelValues behaves like prometheus.CounterVec.WithLabelValues but with type-safe labels.
func (c *CounterVecT[T]) WithLabelValues(labels T) prometheus.Counter {
	return c.inner.WithLabelValues(extractLabelValues(labels)...)
}

// With behaves like prometheus.CounterVec.With but with type-safe labels.
func (c *CounterVecT[T]) With(labels T) prometheus.Counter {
	return c.inner.With(extractLabelsWithValues(labels))
}

// CurryWith behaves like prometheus.CounterVec.CurryWith but with type-safe labels.
// It still returns a CounterVecT, but it's inner prometheus.CounterVec is curried.
func (c *CounterVecT[T]) CurryWith(labels T) (*CounterVecT[T], error) {
	curriedInner, err := c.inner.CurryWith(extractLabelsWithValues(labels))
	if err != nil {
		return nil, err
	}
	c.inner = curriedInner
	return c, nil
}

// MustCurryWith behaves like prometheus.CounterVec.MustCurryWith but with type-safe labels.
// It still returns a CounterVecT, but it's inner prometheus.CounterVec is curried.
func (c *CounterVecT[T]) MustCurryWith(labels T) *CounterVecT[T] {
	c.inner = c.inner.MustCurryWith(extractLabelsWithValues(labels))
	return c
}

// Unsafe returns the underlying prometheus.CounterVec
// it's used to call any other method of prometheus.CounterVec that doesn't require type-safe labels
func (c *CounterVecT[T]) Unsafe() *prometheus.CounterVec {
	return c.inner
}

// NewCounterT simply creates a new prometheus.Counter.
// As it doesn't have any labels, it's already type-safe.
// We keep this method just for consistency and interface fulfillment.
func NewCounterT(opts prometheus.CounterOpts) prometheus.Counter {
	return prometheus.NewCounter(opts)
}

// NewCounterFuncT simply creates a new prometheus.CounterFunc.
// As it doesn't have any labels, it's already type-safe.
// We keep this method just for consistency and interface fulfillment.
func NewCounterFuncT(opts prometheus.CounterOpts, function func() float64) prometheus.CounterFunc {
	return prometheus.NewCounterFunc(opts, function)
}

//
// Shorthand for Metrics with a single label
//

// NewCounterVecT1 creates a new CounterVecT with the only single label
func NewCounterVecT1(opts prometheus.CounterOpts, labelName string) *CounterVecT1 {
	var inner *prometheus.CounterVec
	if factory != nil {
		inner = factory.NewCounterVec(opts, []string{labelName})
	} else {
		inner = prometheus.NewCounterVec(opts, []string{labelName})
	}

	return &CounterVecT1{inner: inner, labelName: labelName}
}

// CounterVecT1 is a wrapper around prometheus.CounterVec that allows a single type-safe label.
type CounterVecT1 struct {
	labelName string
	inner     *prometheus.CounterVec
}

// GetMetricWithLabelValues behaves like prometheus.CounterVec.GetMetricWithLabelValues but with type-safe labels.
func (c *CounterVecT1) GetMetricWithLabelValues(labelValue string) (prometheus.Counter, error) {
	return c.inner.GetMetricWithLabelValues(labelValue)
}

// GetMetricWith behaves like prometheus.CounterVec.GetMetricWith but with type-safe labels.
func (c *CounterVecT1) GetMetricWith(labelValue string) (prometheus.Counter, error) {
	return c.inner.GetMetricWith(prometheus.Labels{c.labelName: labelValue})
}

// WithLabelValues behaves like prometheus.CounterVec.WithLabelValues but with type-safe labels.
func (c *CounterVecT1) WithLabelValues(labelValue string) prometheus.Counter {
	return c.inner.WithLabelValues(labelValue)
}

// With behaves like prometheus.CounterVec.With but with type-safe labels.
func (c *CounterVecT1) With(labelValue string) prometheus.Counter {
	return c.inner.With(prometheus.Labels{c.labelName: labelValue})
}

// CurryWith behaves like prometheus.CounterVec.CurryWith but with type-safe labels.
// It still returns a CounterVecT, but it's inner prometheus.CounterVec is curried.
func (c *CounterVecT1) CurryWith(labelValue string) (*CounterVecT1, error) {
	curriedInner, err := c.inner.CurryWith(prometheus.Labels{c.labelName: labelValue})
	if err != nil {
		return nil, err
	}
	c.inner = curriedInner
	return c, nil
}

// MustCurryWith behaves like prometheus.CounterVec.MustCurryWith but with type-safe labels.
// It still returns a CounterVecT, but it's inner prometheus.CounterVec is curried.
func (c *CounterVecT1) MustCurryWith(labelValue string) *CounterVecT1 {
	c.inner = c.inner.MustCurryWith(prometheus.Labels{c.labelName: labelValue})
	return c
}

// Unsafe returns the underlying prometheus.CounterVec
// it's used to call any other method of prometheus.CounterVec that doesn't require type-safe labels
func (c *CounterVecT1) Unsafe() *prometheus.CounterVec {
	return c.inner
}

//
// Promauto compatibility
//

// Factory is a promauto-like factory that allows type-safe labels.
// We have to duplicate promauto.Factory logic here, because promauto.Factory's registry is private.
type Factory[T labelsProviderMarker] struct {
	r prometheus.Registerer
}

// WithAuto is a helper function that allows to use promauto.With with promsafe.With
func WithAuto[T labelsProviderMarker](r prometheus.Registerer) Factory[T] {
	return Factory[T]{r: r}
}

// NewCounterVecT works like promauto.NewCounterVec but with type-safe labels
func (f Factory[T]) NewCounterVecT(opts prometheus.CounterOpts) *CounterVecT[T] {
	c := NewCounterVecT[T](opts)
	if f.r != nil {
		f.r.MustRegister(c.inner)
	}
	return c
}

// NewCounterT wraps promauto.NewCounter.
// As it doesn't require any labels, it's already type-safe, and we keep it for consistency.
func (f Factory[T]) NewCounterT(opts prometheus.CounterOpts) prometheus.Counter {
	return promauto.With(f.r).NewCounter(opts)
}

// NewCounterFuncT wraps promauto.NewCounterFunc.
// As it doesn't require any labels, it's already type-safe, and we keep it for consistency.
func (f Factory[T]) NewCounterFuncT(opts prometheus.CounterOpts, function func() float64) prometheus.CounterFunc {
	return promauto.With(f.r).NewCounterFunc(opts, function)
}

// TODO: we can't use Factory with NewCounterT1. If we need, then we need a new type-less Factory

//
// Helpers
//

// extractLabelsWithValues extracts labels names+values from a given labelsProviderMarker (parent instance of a StructLabelProvider)
func extractLabelsWithValues(labelProvider labelsProviderMarker) prometheus.Labels {
	if any(labelProvider) == nil {
		return nil
	}

	// TODO: let's handle defaults as well, why not?

	// Here, then, it can be only a struct, that is a parent of StructLabelProvider
	return extractLabelFromStruct(labelProvider)
}

// extractLabelValues extracts label string values from a given labelsProviderMarker (parent instance of aStructLabelProvider)
func extractLabelValues(labelProvider labelsProviderMarker) []string {
	m := extractLabelsWithValues(labelProvider)

	labelValues := make([]string, 0, len(m))
	for _, v := range m {
		labelValues = append(labelValues, v)
	}
	return labelValues
}

// extractLabelNames extracts labels names from a given labelsProviderMarker (parent instance of aStructLabelProvider)
func extractLabelNames(labelProvider labelsProviderMarker) []string {
	if any(labelProvider) == nil {
		return nil
	}

	// Here, then, it can be only a struct, that is a parent of StructLabelProvider
	labels := extractLabelFromStruct(labelProvider)
	labelNames := make([]string, 0, len(labels))
	for k := range labels {
		labelNames = append(labelNames, k)
	}
	return labelNames
}

// extractLabelFromStruct extracts labels names+values from a given StructLabelProvider
func extractLabelFromStruct(structWithLabels any) prometheus.Labels {
	labels := prometheus.Labels{}

	val := reflect.Indirect(reflect.ValueOf(structWithLabels))
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Anonymous {
			continue
		}

		var labelName string
		if ourTag := field.Tag.Get(promsafeTag); ourTag != "" {
			if ourTag == "-" { // tag="-" means "skip this field"
				continue
			}
			labelName = ourTag
		} else {
			labelName = toSnakeCase(field.Name)
		}

		// Note: we don't handle defaults values for now
		// so it can have "nil" values, if you had *string fields, etc
		fieldVal := fmt.Sprintf("%v", val.Field(i).Interface())

		labels[labelName] = fieldVal
	}
	return labels
}

// Convert struct field names to snake_case for Prometheus label compliance.
func toSnakeCase(s string) string {
	s = strings.TrimSpace(s)
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}
