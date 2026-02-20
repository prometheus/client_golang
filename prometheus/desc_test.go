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

func TestNewDescWithUnit_String(t *testing.T) {
	desc := V2.NewDesc(
		"sample_metric_bytes",
		"sample metric with unit",
		UnconstrainedLabels(nil),
		nil,
		WithUnit("bytes"),
	)
	if desc.String() != `Desc{fqName: "sample_metric_bytes", help: "sample metric with unit", unit: "bytes", constLabels: {}, variableLabels: {}}` {
		t.Errorf("String: unexpected output:\ngot:  %s\nwant: %s", desc.String(), desc.String())
	}
}
