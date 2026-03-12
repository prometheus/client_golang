// Copyright 2018 The Prometheus Authors
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

package prometheus

import (
	"testing"
)

func TestNewDescInvalidConstLabelValues(t *testing.T) {
	labelValue := "\xFF"
	desc := NewDesc(
		"sample_label",
		"sample label",
		nil,
		Labels{"a": labelValue},
	)
	if desc.Err() == nil {
		t.Errorf("NewDesc: expected error because const label value is invalid: %s", labelValue)
	}
}

func TestNewDescInvalidVariableLabelName(t *testing.T) {
	labelValue := "__label__"
	desc := NewDesc(
		"sample_label",
		"sample label",
		[]string{labelValue},
		Labels{"a": "b"},
	)
	if desc.Err() == nil {
		t.Errorf("NewDesc: expected error because variable label name is invalid: %s", labelValue)
	}
}

func TestNewDescNilLabelValues(t *testing.T) {
	desc := NewDesc(
		"sample_label",
		"sample label",
		nil,
		nil,
	)
	if desc.Err() != nil {
		t.Errorf("NewDesc: unexpected error: %s", desc.Err())
	}
}

func TestNewDescWithNilLabelValues_String(t *testing.T) {
	desc := NewDesc(
		"sample_label",
		"sample label",
		nil,
		nil,
	)
	if desc.String() != `Desc{fqName: "sample_label", help: "sample label", constLabels: {}, variableLabels: {}}` {
		t.Errorf("String: unexpected output: %s", desc.String())
	}
}

func TestNewInvalidDesc_String(t *testing.T) {
	desc := NewInvalidDesc(
		nil,
	)
	if desc.String() != `Desc{fqName: "", help: "", constLabels: {}, variableLabels: {}}` {
		t.Errorf("String: unexpected output: %s", desc.String())
	}
}

func TestDesc_Name(t *testing.T) {
	tests := []struct {
		name     string
		fqName   string
		expected string
	}{
		{
			name:     "simple name",
			fqName:   "my_metric",
			expected: "my_metric",
		},
		{
			name:     "namespaced metric",
			fqName:   "namespace_subsystem_metric_name",
			expected: "namespace_subsystem_metric_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := NewDesc(tt.fqName, "help text", nil, nil)
			if got := desc.Name(); got != tt.expected {
				t.Errorf("Name() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDesc_Help(t *testing.T) {
	tests := []struct {
		name     string
		help     string
		expected string
	}{
		{
			name:     "simple help",
			help:     "This is a help string",
			expected: "This is a help string",
		},
		{
			name:     "empty help",
			help:     "",
			expected: "",
		},
		{
			name:     "multiline help",
			help:     "Line 1\nLine 2",
			expected: "Line 1\nLine 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := NewDesc("my_metric", tt.help, nil, nil)
			if got := desc.Help(); got != tt.expected {
				t.Errorf("Help() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDesc_ConstLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   Labels
		expected Labels
	}{
		{
			name:     "no labels",
			labels:   nil,
			expected: Labels{},
		},
		{
			name:     "empty labels",
			labels:   Labels{},
			expected: Labels{},
		},
		{
			name:     "single label",
			labels:   Labels{"env": "prod"},
			expected: Labels{"env": "prod"},
		},
		{
			name: "multiple labels",
			labels: Labels{
				"env":     "prod",
				"region":  "us-west",
				"version": "1.0",
			},
			expected: Labels{
				"env":     "prod",
				"region":  "us-west",
				"version": "1.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := NewDesc("my_metric", "help", nil, tt.labels)
			got := desc.ConstLabels()

			// Check length
			if len(got) != len(tt.expected) {
				t.Errorf("ConstLabels() length = %d, want %d", len(got), len(tt.expected))
			}

			// Check all expected labels are present with correct values
			for k, v := range tt.expected {
				if gotVal, ok := got[k]; !ok {
					t.Errorf("ConstLabels() missing key %q", k)
				} else if gotVal != v {
					t.Errorf("ConstLabels()[%q] = %q, want %q", k, gotVal, v)
				}
			}

			// Ensure returned map is a copy (modifying it shouldn't affect the descriptor)
			if len(got) > 0 {
				got["test_key"] = "test_value"
				got2 := desc.ConstLabels()
				if _, exists := got2["test_key"]; exists {
					t.Error("ConstLabels() should return a copy, not the internal map")
				}
			}
		})
	}
}

func TestDesc_VariableLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   []string
		expected []string
	}{
		{
			name:     "no labels",
			labels:   nil,
			expected: nil,
		},
		{
			name:     "empty labels",
			labels:   []string{},
			expected: nil,
		},
		{
			name:     "single label",
			labels:   []string{"method"},
			expected: []string{"method"},
		},
		{
			name:     "multiple labels",
			labels:   []string{"method", "status", "path"},
			expected: []string{"method", "status", "path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := NewDesc("my_metric", "help", tt.labels, nil)
			got := desc.VariableLabels()

			// Check nil vs non-nil
			if (got == nil) != (tt.expected == nil) {
				t.Errorf("VariableLabels() = %v (nil=%t), want %v (nil=%t)",
					got, got == nil, tt.expected, tt.expected == nil)
				return
			}

			// If both are nil, test passes
			if got == nil && tt.expected == nil {
				return
			}

			// Check length
			if len(got) != len(tt.expected) {
				t.Errorf("VariableLabels() length = %d, want %d", len(got), len(tt.expected))
			}

			// Check all labels are present in order
			for i, label := range tt.expected {
				if i >= len(got) || got[i] != label {
					t.Errorf("VariableLabels()[%d] = %q, want %q", i, got[i], label)
				}
			}

			// Ensure returned slice is a copy (modifying it shouldn't affect the descriptor)
			if len(got) > 0 {
				got[0] = "modified"
				got2 := desc.VariableLabels()
				if got2[0] == "modified" {
					t.Error("VariableLabels() should return a copy, not the internal slice")
				}
			}
		})
	}
}

func TestDesc_GettersComprehensive(t *testing.T) {
	// Create a descriptor with all fields populated
	desc := NewDesc(
		"my_namespace_my_subsystem_my_metric",
		"This is a comprehensive help text",
		[]string{"label1", "label2", "label3"},
		Labels{"const1": "value1", "const2": "value2"},
	)

	if desc.Err() != nil {
		t.Fatalf("Unexpected error creating desc: %v", desc.Err())
	}

	// Test Name()
	if got := desc.Name(); got != "my_namespace_my_subsystem_my_metric" {
		t.Errorf("Name() = %q, want %q", got, "my_namespace_my_subsystem_my_metric")
	}

	// Test Help()
	if got := desc.Help(); got != "This is a comprehensive help text" {
		t.Errorf("Help() = %q, want %q", got, "This is a comprehensive help text")
	}

	// Test ConstLabels()
	constLabels := desc.ConstLabels()
	if len(constLabels) != 2 {
		t.Errorf("ConstLabels() len = %d, want 2", len(constLabels))
	}
	if constLabels["const1"] != "value1" {
		t.Errorf("ConstLabels()[const1] = %q, want %q", constLabels["const1"], "value1")
	}
	if constLabels["const2"] != "value2" {
		t.Errorf("ConstLabels()[const2] = %q, want %q", constLabels["const2"], "value2")
	}

	// Test VariableLabels()
	varLabels := desc.VariableLabels()
	if len(varLabels) != 3 {
		t.Errorf("VariableLabels() len = %d, want 3", len(varLabels))
	}
	expectedVarLabels := []string{"label1", "label2", "label3"}
	for i, expected := range expectedVarLabels {
		if varLabels[i] != expected {
			t.Errorf("VariableLabels()[%d] = %q, want %q", i, varLabels[i], expected)
		}
	}
}
