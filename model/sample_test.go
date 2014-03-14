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
	"sort"
	"testing"
)

func TestSamplesSort(t *testing.T) {
	input := Samples{
		&Sample{
			// Fingerprint: 7658840550047852795
			Metric: Metric{
				MetricNameLabel: "B",
			},
			Timestamp: 1,
		},
		&Sample{
			// Fingerprint: 7658840550047852795
			Metric: Metric{
				MetricNameLabel: "B",
			},
			Timestamp: 2,
		},
		&Sample{
			// Fingerprint: 13206712341014817019
			Metric: Metric{
				MetricNameLabel: "A",
			},
			Timestamp: 1,
		},
		&Sample{
			// Fingerprint: 13206712341014817019
			Metric: Metric{
				MetricNameLabel: "A",
			},
			Timestamp: 2,
		},
		&Sample{
			// Fingerprint: 2110968759080888571
			Metric: Metric{
				MetricNameLabel: "C",
			},
			Timestamp: 1,
		},
		&Sample{
			// Fingerprint: 2110968759080888571
			Metric: Metric{
				MetricNameLabel: "C",
			},
			Timestamp: 2,
		},
	}

	expected := Samples{
		&Sample{
			// Fingerprint: 2110968759080888571
			Metric: Metric{
				MetricNameLabel: "C",
			},
			Timestamp: 1,
		},
		&Sample{
			// Fingerprint: 2110968759080888571
			Metric: Metric{
				MetricNameLabel: "C",
			},
			Timestamp: 2,
		},
		&Sample{
			// Fingerprint: 7658840550047852795
			Metric: Metric{
				MetricNameLabel: "B",
			},
			Timestamp: 1,
		},
		&Sample{
			// Fingerprint: 7658840550047852795
			Metric: Metric{
				MetricNameLabel: "B",
			},
			Timestamp: 2,
		},
		&Sample{
			// Fingerprint: 13206712341014817019
			Metric: Metric{
				MetricNameLabel: "A",
			},
			Timestamp: 1,
		},
		&Sample{
			// Fingerprint: 13206712341014817019
			Metric: Metric{
				MetricNameLabel: "A",
			},
			Timestamp: 2,
		},
	}

	sort.Sort(input)

	for i, actual := range input {
		actualFp := Fingerprint{}
		actualFp.LoadFromMetric(actual.Metric)

		expectedFp := Fingerprint{}
		expectedFp.LoadFromMetric(expected[i].Metric)

		if !actualFp.Equal(&expectedFp) {
			t.Fatalf("%d. Incorrect fingerprint. Got %s; want %s", i, actualFp, expectedFp)
		}

		if actual.Timestamp != expected[i].Timestamp {
			t.Fatalf("%d. Incorrect timestamp. Got %s; want %s", i, actual.Timestamp, expected[i].Timestamp)
		}
	}
}
