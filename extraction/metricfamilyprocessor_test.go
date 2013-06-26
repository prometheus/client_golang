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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/model"
)

var testTime = time.Now()

type metricFamilyProcessorScenario struct {
	in  string
	out []*Result
}

func (s *metricFamilyProcessorScenario) test(t *testing.T, set int) {
	i := strings.NewReader(s.in)
	chanSize := 1
	if len(s.out) > 0 {
		chanSize = len(s.out) * 3
	}
	r := make(chan *Result, chanSize)

	o := &ProcessOptions{
		Timestamp:  testTime,
		BaseLabels: model.LabelSet{"base": "label"},
	}

	err := MetricFamilyProcessor.ProcessSingle(i, r, o)
	if err != nil {
		t.Fatalf("%d. got error: %s", set, err)
	}
	close(r)

	actual := []*Result{}

	for e := range r {
		actual = append(actual, e)
	}

	if len(actual) != len(s.out) {
		t.Fatalf("%d. expected length %d, got %d", set, len(s.out), len(actual))
	}

	for i, expected := range s.out {
		if expected.Err != actual[i].Err {
			t.Fatalf("%d. expected err of %s, got %s", set, expected.Err, actual[i].Err)
		}

		if len(expected.Samples) != len(actual[i].Samples) {
			t.Fatalf("%d.%d expected %d samples, got %d", set, i, len(expected.Samples), len(actual[i].Samples))
		}

		for j := 0; j < len(expected.Samples); j++ {
			e := expected.Samples[j]
			a := actual[i].Samples[j]
			if !a.Equal(e) {
				t.Fatalf("%d.%d.%d expected %s sample, got %s", set, i, j, e, a)
			}
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
			out: []*Result{
				{
					Samples: model.Samples{
						&model.Sample{
							Metric:    model.Metric{"base": "label", "name": "request_count", "some_label_name": "some_label_value"},
							Value:     -42,
							Timestamp: testTime,
						},
						&model.Sample{
							Metric:    model.Metric{"base": "label", "name": "request_count", "another_label_name": "another_label_value"},
							Value:     84,
							Timestamp: testTime,
						},
					},
				},
			},
		},
		{
			in: "\xb9\x01\n\rrequest_count\x12\x12Number of requests\x18\x02\"O\n#\n\x0fsome_label_name\x12\x10some_label_value\"(\x1a\x12\t\xaeG\xe1z\x14\xae\xef?\x11\x00\x00\x00\x00\x00\x00E\xc0\x1a\x12\t+\x87\x16\xd9\xce\xf7\xef?\x11\x00\x00\x00\x00\x00\x00U\xc0\"A\n)\n\x12another_label_name\x12\x13another_label_value\"\x14\x1a\x12\t\x00\x00\x00\x00\x00\x00\xe0?\x11\x00\x00\x00\x00\x00\x00$@",
			out: []*Result{
				{
					Samples: model.Samples{
						&model.Sample{
							Metric:    model.Metric{"base": "label", "name": "request_count", "some_label_name": "some_label_value", "quantile": "0.99"},
							Value:     -42,
							Timestamp: testTime,
						},
						&model.Sample{
							Metric:    model.Metric{"base": "label", "name": "request_count", "some_label_name": "some_label_value", "quantile": "0.999"},
							Value:     -84,
							Timestamp: testTime,
						},
						&model.Sample{
							Metric:    model.Metric{"base": "label", "name": "request_count", "another_label_name": "another_label_value", "quantile": "0.5"},
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
