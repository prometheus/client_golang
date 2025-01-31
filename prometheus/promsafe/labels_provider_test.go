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
	"reflect"
	"strconv"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// CustomType that will be stringified via String() method
type CustomType int

const (
	CustomConst0 CustomType = iota
	CustomConst1
	CustomConst2
)

func (ct CustomType) String() string {
	switch ct {
	case CustomConst1:
		return "c1"
	case CustomConst2:
		return "c2"

	default:
		return "c0"
	}
}

// CustomTypeInt will remain as simple int
type CustomTypeInt int

const (
	CustomConstInt100 CustomTypeInt = 100
	CustomConstInt200 CustomTypeInt = 200
)

type TestLabels struct {
	StructLabelProvider
	Field1 string
	Field2 int
	Field3 bool

	Field4 CustomType
	Field5 CustomTypeInt
}

type TestLabelsWithTags struct {
	StructLabelProvider
	FieldA string `promsafe:"custom_a"`
	FieldB int    `promsafe:"-"`
	FieldC bool   // no promsafe tag, so default to snake_case of field_c
}

type TestLabelsWithPointers struct {
	StructLabelProvider
	Field1 *string
	Field2 *int
	Field3 *bool
}

type TestLabelsFast struct {
	StructLabelProvider
	Field1 string
	Field2 int
	Field3 bool
}

func (t TestLabelsFast) ToPrometheusLabels() prometheus.Labels {
	return prometheus.Labels{
		"f1": t.Field1,
		"f2": strconv.Itoa(t.Field2),
		"f3": strconv.FormatBool(t.Field3),
	}
}

func (t TestLabelsFast) ToLabelNames() []string {
	return []string{"f1", "f2", "f3"}
}

func Test_extractLabelsWithValues(t *testing.T) {
	tests := []struct {
		name       string
		input      LabelsProviderMarker
		expected   prometheus.Labels
		shouldFail bool
	}{
		{
			name: "Basic struct without custom tags",
			input: TestLabels{
				Field1: "value1",
				Field2: 123,
				Field3: true,
				Field4: CustomConst1,
				Field5: CustomConstInt200,
			},
			expected: prometheus.Labels{
				"field1": "value1",
				"field2": "123",
				"field3": "true",
				"field4": "c1",
				"field5": "200",
			},
		},
		{
			name: "Struct with custom tags and exclusions",
			input: TestLabelsWithTags{
				FieldA: "customValue",
				FieldB: 456,
				FieldC: false,
			},
			expected: prometheus.Labels{
				"custom_a": "customValue",
				"field_c":  "false",
			},
		},
		{
			name: "Struct with pointers",
			input: TestLabelsWithPointers{
				Field1: ptr("ptrValue"),
				Field2: ptr(789),
				Field3: ptr(true),
			},
			expected: prometheus.Labels{
				"field1": "ptrValue",
				"field2": "789",
				"field3": "true",
			},
		},
		{
			name: "Struct fast (with declared methods)",
			input: TestLabelsFast{
				Field1: "hello",
				Field2: 100,
				Field3: true,
			},
			expected: prometheus.Labels{
				"f1": "hello",
				"f2": "100",
				"f3": "true",
			},
		},
		{
			name:     "Nil will return empty result",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldFail {
						t.Errorf("unexpected panic: %v", r)
					}
				}
			}()

			// Call extractLabelsFromStruct
			got := extractLabelsWithValues(tt.input)

			// Compare results
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("extractLabelFromStruct(%v) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func Test_extractLabelNames(t *testing.T) {
	tests := []struct {
		name       string
		input      LabelsProviderMarker
		expected   []string
		shouldFail bool
	}{
		{
			name: "Basic struct without custom tags",
			input: TestLabels{
				Field1: "value1",
				Field2: 123,
				Field3: true,
			},
			expected: []string{"field1", "field2", "field3"},
		},
		{
			name: "Struct with custom tags and exclusions",
			input: TestLabelsWithTags{
				FieldA: "customValue",
				FieldB: 456,
				FieldC: false,
			},
			expected: []string{"custom_a", "field_c"},
		},
		{
			name: "Struct with pointers",
			input: TestLabelsWithPointers{
				Field1: ptr("ptrValue"),
				Field2: ptr(789),
				Field3: ptr(true),
			},
			expected: []string{"field1", "field2", "field3"},
		},
		{
			name: "Struct fast (with declared methods)",
			input: TestLabelsFast{
				Field1: "hello",
				Field2: 100,
				Field3: true,
			},
			expected: []string{"f1", "f2", "f3"},
		},
		{
			name:     "Nil will return empty result",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.shouldFail {
						t.Errorf("unexpected panic: %v", r)
					}
				}
			}()

			// Call extractLabelsFromStruct
			got := extractLabelNames(tt.input)

			// Compare results
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("extractLabelFromStruct(%v) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func Test_NewEmptyLabels(t *testing.T) {
	got1 := NewEmptyLabels[TestLabels]()
	if !reflect.DeepEqual(got1, TestLabels{}) {
		t.Errorf("NewEmptyLabels[%T] = %v; want %v", TestLabels{}, got1, TestLabels{})
	}
	got2 := NewEmptyLabels[TestLabelsWithTags]()
	if !reflect.DeepEqual(got2, TestLabelsWithTags{}) {
		t.Errorf("NewEmptyLabels[%T] = %v; want %v", TestLabelsWithTags{}, got1, TestLabelsWithTags{})
	}
	got3 := NewEmptyLabels[TestLabelsWithPointers]()
	if !reflect.DeepEqual(got3, TestLabelsWithPointers{}) {
		t.Errorf("NewEmptyLabels[%T] = %v; want %v", TestLabelsWithPointers{}, got1, TestLabelsWithPointers{})
	}
	got4 := NewEmptyLabels[*TestLabelsFast]()
	if !reflect.DeepEqual(*got4, TestLabelsFast{}) {
		t.Errorf("NewEmptyLabels[%T] = %v; want %v", TestLabelsFast{}, *got4, TestLabelsFast{})
	}
}

func Test_SetPromsafeTag(t *testing.T) {
	SetPromsafeTag("prom")
	defer func() {
		SetPromsafeTag("")
	}()
	if promsafeTag != "prom" {
		t.Errorf("promsafeTag = %v; want %v", promsafeTag, "prom")
	}

	type CustomTestLabels struct {
		StructLabelProvider
		FieldX string `prom:"x"`
	}

	extractedLabelNames := extractLabelNames(CustomTestLabels{})
	if !reflect.DeepEqual(extractedLabelNames, []string{"x"}) {
		t.Errorf("Using custom promsafeTag: extractLabelNames(%v) = %v; want %v", CustomTestLabels{}, extractedLabelNames, []string{"x"})
	}
}

// Helper functions to create pointers
func ptr[T any](v T) *T {
	return &v
}
