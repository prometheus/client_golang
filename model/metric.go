// Copyright 2013 Prometheus Team
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

package model

import (
	"fmt"
	"sort"
	"strings"
)

// A Metric is similar to a LabelSet, but the key difference is that a Metric is
// a singleton and refers to one and only one stream of samples.
type Metric map[LabelName]LabelValue

// Equal compares the fingerprints of both metrics.
func (m Metric) Equal(o Metric) bool {
	return m.Fingerprint().Equal(o.Fingerprint())
}

// Before compares the fingerprints of both metrics.
func (m Metric) Before(o Metric) bool {
	return m.Fingerprint().Less(o.Fingerprint())
}

func (m Metric) String() string {
	metricName, hasName := m[MetricNameLabel]
	numLabels := len(m) - 1
	if !hasName {
		numLabels = len(m)
	}
	labelStrings := make([]string, 0, numLabels)
	for label, value := range m {
		if label != MetricNameLabel {
			labelStrings = append(labelStrings, fmt.Sprintf("%s=%q", label, value))
		}
	}

	switch numLabels {
	case 0:
		if hasName {
			return string(metricName)
		} else {
			return "{}"
		}
	default:
		sort.Strings(labelStrings)
		return fmt.Sprintf("%s{%s}", metricName, strings.Join(labelStrings, ", "))
	}
}

func (m Metric) Fingerprint() Fingerprint {
	var fp Fingerprint
	fp.LoadFromMetric(m)
	return fp
}

// MergeFromLabelSet merges a label set into this Metric, prefixing a collision
// prefix to the label names merged from the label set where required.
func (m Metric) MergeFromLabelSet(labels LabelSet, collisionPrefix LabelName) {
	for k, v := range labels {
		if collisionPrefix != "" {
			for {
				if _, exists := m[k]; !exists {
					break
				}
				k = collisionPrefix + k
			}
		}

		m[k] = v
	}
}
