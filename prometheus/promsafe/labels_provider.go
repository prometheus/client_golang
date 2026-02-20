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

package promsafe

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// LabelsProvider is an interface that allows to convert anything into prometheus.Labels
// It allows to provide your own FAST implementation of Struct->prometheus.Labels conversion
// without using reflection.
type LabelsProvider interface {
	ToPrometheusLabels() prometheus.Labels
	ToLabelNames() []string
}

// LabelsProviderMarker is a marker interface for enforcing type-safety of StructLabelProvider.
type LabelsProviderMarker interface {
	labelsProviderMarker()
}

// StructLabelProvider should be embedded in any struct that serves as a label provider.
type StructLabelProvider struct{}

var _ LabelsProviderMarker = (*StructLabelProvider)(nil)

func (s StructLabelProvider) labelsProviderMarker() {
	panic("LabelsProviderMarker interface method should never be called")
}

// NewEmptyLabels creates a new empty labels instance of type T
// It's a bit tricky as we want to support both structs and pointers to structs
// e.g. &MyLabels{StructLabelProvider} or MyLabels{StructLabelProvider}
func NewEmptyLabels[T LabelsProviderMarker]() T {
	var emptyLabels T

	val := reflect.ValueOf(&emptyLabels).Elem()
	if val.Kind() == reflect.Ptr {
		ptrType := val.Type().Elem()
		newValue := reflect.New(ptrType).Interface().(T)
		return newValue
	}

	return emptyLabels
}

//
// Helpers
//

// promsafeTag is the tag name used for promsafe labels inside structs.
// The tag is optional, as if not present, field is used with snake_cased FieldName.
// It's useful to use a tag when you want to override the default naming or exclude a field from the metric.
var promsafeTag = "promsafe"

// SetPromsafeTag sets the tag name used for promsafe labels inside structs.
func SetPromsafeTag(tag string) {
	promsafeTag = tag
}

// iterateStructFields iterates over struct fields, calling the given function for each field.
func iterateStructFields(structValue any, fn func(labelName string, fieldValue reflect.Value)) {
	val := reflect.Indirect(reflect.ValueOf(structValue))
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Anonymous {
			continue
		}

		// Handle tag logic centrally
		var labelName string
		if ourTag := field.Tag.Get(promsafeTag); ourTag == "-" {
			continue // Skip field
		} else if ourTag != "" {
			labelName = ourTag
		} else {
			labelName = toSnakeCase(field.Name)
		}

		fn(labelName, val.Field(i))
	}
}

// extractLabelsWithValues extracts labels names+values from a given LabelsProviderMarker (parent instance of a StructLabelProvider)
func extractLabelsWithValues(labelProvider LabelsProviderMarker) prometheus.Labels {
	if any(labelProvider) == nil {
		return nil
	}

	if clp, ok := labelProvider.(LabelsProvider); ok {
		return clp.ToPrometheusLabels()
	}

	// extracting labels from a struct
	labels := prometheus.Labels{}
	iterateStructFields(labelProvider, func(labelName string, fieldValue reflect.Value) {
		labels[labelName] = stringifyLabelValue(fieldValue)
	})
	return labels
}

// extractLabelNames extracts labels names from a given LabelsProviderMarker (parent instance of aStructLabelProvider)
func extractLabelNames(labelProvider LabelsProviderMarker) []string {
	if any(labelProvider) == nil {
		return nil
	}

	// If custom implementation is done, just do it
	if lp, ok := labelProvider.(LabelsProvider); ok {
		return lp.ToLabelNames()
	}

	// Fallback to slow implementation via reflect
	// Important! We return label names in order of fields in the struct
	labelNames := make([]string, 0)
	iterateStructFields(labelProvider, func(labelName string, fieldValue reflect.Value) {
		labelNames = append(labelNames, labelName)
	})

	return labelNames
}

// stringifyLabelValue makes up a valid string value from a given field's value
// It's used ONLY in fallback reflect mode
// Field value might be a pointer, that's why we do reflect.Indirect()
// Note: in future we can handle default values here as well
func stringifyLabelValue(v reflect.Value) string {
	// TODO: we probably want to handle custom type processing here
	//       e.g. sometimes booleans need to be "on"/"off" instead of "true"/"false"
	return fmt.Sprintf("%v", reflect.Indirect(v).Interface())
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
