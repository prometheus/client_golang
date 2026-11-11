// Copyright The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promlint_test

import (
	"testing"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus/testutil/promlint"
)

// A MetricFamily without a name is a legal protobuf message. The linter's
// public NewWithMetricFamilies API must report problems, not panic. Every
// default validation except LintMetricUnits reads the name via GetName(),
// which is nil-safe.
func TestLintMetricFamilyWithoutNameDoesNotPanic(t *testing.T) {
	mfs := []*dto.MetricFamily{
		{}, // Name == nil
	}

	l := promlint.NewWithMetricFamilies(mfs)
	if _, err := l.Lint(); err != nil {
		t.Fatalf("Lint returned an unexpected error: %v", err)
	}
}
