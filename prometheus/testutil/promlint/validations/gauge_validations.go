// Copyright 2024 The Prometheus Authors
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

package validations

import (
	"errors"
	"strings"

	dto "github.com/prometheus/client_model/go"
)

// LintGauge detects issues specific to gauges, as well as patterns that should
// only be used with gauges.
func LintGauge(mf *dto.MetricFamily) []error {
	var problems []error

	isGauge := mf.GetType() == dto.MetricType_GAUGE
	isUntyped := mf.GetType() == dto.MetricType_UNTYPED
	hasTimestampSecondsSuffix := strings.HasSuffix(mf.GetName(), "_timestamp_seconds")

	if !isUntyped && !isGauge && hasTimestampSecondsSuffix {
		problems = append(problems, errors.New(`non-gauge metrics should not have "_timestamp_seconds" suffix`))
	}

	return problems
}
