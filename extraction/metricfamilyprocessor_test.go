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

package extraction

import (
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/model"
)

var testTime = model.Now()

type metricFamilyProcessorScenario struct {
	in               string
	expected, actual []*Result
}

func (s *metricFamilyProcessorScenario) Ingest(r *Result) error {
	s.actual = append(s.actual, r)
	return nil
}

func (s *metricFamilyProcessorScenario) test(t *testing.T, set int) {
	i := strings.NewReader(s.in)

	o := &ProcessOptions{
		Timestamp: testTime,
	}

	err := MetricFamilyProcessor.ProcessSingle(i, s, o)
	if err != nil {
		t.Fatalf("%d. got error: %s", set, err)
	}

	if len(s.expected) != len(s.actual) {
		t.Fatalf("%d. expected length %d, got %d", set, len(s.expected), len(s.actual))
	}

	for i, expected := range s.expected {
		sort.Sort(s.actual[i].Samples)
		sort.Sort(expected.Samples)

		if !expected.equal(s.actual[i]) {
			t.Errorf("%d.%d. expected %s, got %s", set, i, expected, s.actual[i])
		}
	}
}

func TestMetricFamilyProcessor(t *testing.T) {
	scenarios := []metricFamilyProcessorScenario{
		{
			in: "",
		},
		{
			in: "\x8f\x01\n\rrequest_count\x12\x12Number of requests\x18\x00\"0\n#\n\x0fsome_label_name\x12\x10some_label_value\x1a\t\t\x00\x00\x00\x00\x00\x00E\xc0\"6\n)\n\x12another_label_name\x12\x13another_label_value\x1a\t\t\x00\x00\x00\x00\x00\x00U@",
			expected: []*Result{
				{
					Samples: model.Samples{
						&model.Sample{
							Metric:    model.Metric{model.MetricNameLabel: "request_count", "some_label_name": "some_label_value"},
							Value:     -42,
							Timestamp: testTime,
						},
						&model.Sample{
							Metric:    model.Metric{model.MetricNameLabel: "request_count", "another_label_name": "another_label_value"},
							Value:     84,
							Timestamp: testTime,
						},
					},
				},
			},
		},
		{
			in: "\xb9\x01\n\rrequest_count\x12\x12Number of requests\x18\x02\"O\n#\n\x0fsome_label_name\x12\x10some_label_value\"(\x1a\x12\t\xaeG\xe1z\x14\xae\xef?\x11\x00\x00\x00\x00\x00\x00E\xc0\x1a\x12\t+\x87\x16\xd9\xce\xf7\xef?\x11\x00\x00\x00\x00\x00\x00U\xc0\"A\n)\n\x12another_label_name\x12\x13another_label_value\"\x14\x1a\x12\t\x00\x00\x00\x00\x00\x00\xe0?\x11\x00\x00\x00\x00\x00\x00$@",
			expected: []*Result{
				{
					Samples: model.Samples{
						&model.Sample{
							Metric:    model.Metric{model.MetricNameLabel: "request_count", "some_label_name": "some_label_value", "quantile": "0.99"},
							Value:     -42,
							Timestamp: testTime,
						},
						&model.Sample{
							Metric:    model.Metric{model.MetricNameLabel: "request_count", "some_label_name": "some_label_value", "quantile": "0.999"},
							Value:     -84,
							Timestamp: testTime,
						},
						&model.Sample{
							Metric:    model.Metric{model.MetricNameLabel: "request_count", "another_label_name": "another_label_value", "quantile": "0.5"},
							Value:     10,
							Timestamp: testTime,
						},
					},
				},
			},
		},
	}

	for i, scenario := range scenarios {
		scenario.test(t, i)
	}
}
