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
	"slices"
	"strconv"
	"strings"

	dto "github.com/prometheus/client_model/go"
)

// LintDuplicateMetric detects duplicate metric.
func LintDuplicateMetric(mf *dto.MetricFamily) []error {
	var problems []error

	seen := make(map[string]struct{}, len(mf.Metric))
	for _, m := range mf.Metric {
		// The order of the labels is irrelevant for the identity of a metric.
		pairs := make([]string, 0, len(m.Label))
		for _, l := range m.Label {
			// An empty label value is equivalent to the label being absent.
			if l.GetValue() == "" {
				continue
			}
			pairs = append(pairs, strconv.Quote(l.GetName())+"="+strconv.Quote(l.GetValue()))
		}
		slices.Sort(pairs)
		key := strings.Join(pairs, ",")
		if _, ok := seen[key]; ok {
			problems = append(problems, errors.New("metric not unique"))
			continue
		}
		seen[key] = struct{}{}
	}

	return problems
}
