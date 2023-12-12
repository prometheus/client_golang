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

func TestNewDescInvalidLabelValues(t *testing.T) {
	desc := NewDesc(
		"sample_label",
		"sample label",
		nil,
		Labels{"a": "\xFF"},
	)
	if desc.err == nil {
		t.Errorf("NewDesc: expected error because: %s", desc.err)
	}
}

func TestNewDescUTF8(t *testing.T) {
	model.NameValidationScheme = model.UTF8Validation
	desc := NewDesc(
		"sample.label",
		"sample label",
		nil,
		Labels{"a": "whatever"},
	)
	model.NameValidationScheme = model.LegacyValidation
	if desc.err != nil {
		t.Errorf("NewDesc: expected no error but got: %s", desc.err)
	}
}
