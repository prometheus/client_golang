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

	"github.com/prometheus/common/model"
)

func TestNewDesc(t *testing.T) {
	testCases := []struct {
		name           string
		fqName         string
		help           string
		variableLabels []string
		labels         Labels
		opts           []DescOption
		wantErr        string
	}{
		{
			name:           "invalid label value",
			fqName:         "sample_label",
			help:           "sample label",
			variableLabels: nil,
			labels:         Labels{"a": "\xff"},
			wantErr:        `label value "\xff" is not valid UTF-8`,
		},
		{
			name:           "nil label values",
			fqName:         "sample_label",
			help:           "sample label",
			variableLabels: nil,
			labels:         nil,
		},
		{
			name:           "invalid label name",
			fqName:         "sample_label",
			help:           "sample label",
			variableLabels: nil,
			labels:         Labels{"\xff": "test"},
			wantErr:        `"\xff" is not a valid label name for metric "sample_label"`,
		},
		{
			name:           "invalid legacy label name",
			fqName:         "sample_label",
			help:           "sample label",
			variableLabels: nil,
			labels:         Labels{"test😀": "test"},
			opts:           []DescOption{WithValidationScheme(model.LegacyValidation)},
			wantErr:        `"test😀" is not a valid label name for metric "sample_label"`,
		},
		{
			name:    "invalid legacy metric name",
			fqName:  "sample_label😀",
			help:    "sample label",
			opts:    []DescOption{WithValidationScheme(model.LegacyValidation)},
			wantErr: `"sample_label😀" is not a valid metric name`,
		},
		{
			name:           "valid utf8 label name",
			fqName:         "sample_label",
			help:           "sample label",
			variableLabels: nil,
			labels:         Labels{"test😀": "test"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			desc := NewDesc(
				tc.fqName,
				tc.help,
				tc.variableLabels,
				tc.labels,
				tc.opts...,
			)
			if desc.err != nil && tc.wantErr != desc.err.Error() {
				t.Fatalf("NewDesc: expected error %q but got %+v", tc.wantErr, desc.err)
			} else if desc.err == nil && tc.wantErr != "" {
				t.Fatalf("NewDesc: expected error %q but got nil", tc.wantErr)
			} else if desc.err != nil && tc.wantErr == "" {
				t.Fatalf("NewDesc: %+v", desc.err)
			}
		})
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
