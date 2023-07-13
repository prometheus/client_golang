// Copyright 2020 The Prometheus Authors
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
	"fmt"
	"regexp"
	"strings"

	dto "github.com/prometheus/client_model/go"
)

var camelCase = regexp.MustCompile(`[a-z][A-Z]`)

// lintMetricUnits detects issues with metric unit names.
func lintMetricUnits(mf *dto.MetricFamily) []Problem {
	var problems []Problem

	unit, base, ok := metricUnits(*mf.Name)
	if !ok {
		// No known units detected.
		return nil
	}

	// Unit is already a base unit.
	if unit == base {
		return nil
	}

	problems = append(problems, newProblem(mf, fmt.Sprintf("use base unit %q instead of %q", base, unit)))

	return problems
}

// lintMetricTypeInName detects when metric types are included in the metric name.
func lintMetricTypeInName(mf *dto.MetricFamily) []Problem {
	var problems []Problem
	n := strings.ToLower(mf.GetName())

	for i, t := range dto.MetricType_name {
		if i == int32(dto.MetricType_UNTYPED) {
			continue
		}

		typename := strings.ToLower(t)
		if strings.Contains(n, "_"+typename+"_") || strings.HasSuffix(n, "_"+typename) {
			problems = append(problems, newProblem(mf, fmt.Sprintf(`metric name should not include type '%s'`, typename)))
		}
	}
	return problems
}

// lintReservedChars detects colons in metric names.
func lintReservedChars(mf *dto.MetricFamily) []Problem {
	var problems []Problem
	if strings.Contains(mf.GetName(), ":") {
		problems = append(problems, newProblem(mf, "metric names should not contain ':'"))
	}
	return problems
}

// lintCamelCase detects metric names and label names written in camelCase.
func lintCamelCase(mf *dto.MetricFamily) []Problem {
	var problems []Problem
	if camelCase.FindString(mf.GetName()) != "" {
		problems = append(problems, newProblem(mf, "metric names should be written in 'snake_case' not 'camelCase'"))
	}

	for _, m := range mf.GetMetric() {
		for _, l := range m.GetLabel() {
			if camelCase.FindString(l.GetName()) != "" {
				problems = append(problems, newProblem(mf, "label names should be written in 'snake_case' not 'camelCase'"))
			}
		}
	}
	return problems
}

// lintUnitAbbreviations detects abbreviated units in the metric name.
func lintUnitAbbreviations(mf *dto.MetricFamily) []Problem {
	var problems []Problem
	n := strings.ToLower(mf.GetName())
	for _, s := range unitAbbreviations {
		if strings.Contains(n, "_"+s+"_") || strings.HasSuffix(n, "_"+s) {
			problems = append(problems, newProblem(mf, "metric names should not contain abbreviated units"))
		}
	}
	return problems
}
