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

package decoding

import (
	"container/list"
	"fmt"
	"os"
	"path"
	"runtime"
	"testing"
	"time"
)

func testProcessor002Process(t tester) {
	var scenarios = []struct {
		in         string
		baseLabels LabelSet
		out        Samples
		err        error
	}{
		{
			in:  "empty.json",
			err: fmt.Errorf("EOF"),
		},
		{
			in: "test0_0_1-0_0_2.json",
			baseLabels: LabelSet{
				JobLabel: "batch_exporter",
			},
			out: Samples{
				&Sample{
					Metric: Metric{"service": "zed", MetricNameLabel: "rpc_calls_total", "job": "batch_job", "exporter_job": "batch_exporter"},
					Value:  25,
				},
				&Sample{
					Metric: Metric{"service": "bar", MetricNameLabel: "rpc_calls_total", "job": "batch_job", "exporter_job": "batch_exporter"},
					Value:  25,
				},
				&Sample{
					Metric: Metric{"service": "foo", MetricNameLabel: "rpc_calls_total", "job": "batch_job", "exporter_job": "batch_exporter"},
					Value:  25,
				},
				&Sample{
					Metric: Metric{"percentile": "0.010000", MetricNameLabel: "rpc_latency_microseconds", "service": "zed", "job": "batch_exporter"},
					Value:  0.0459814091918713,
				},
				&Sample{
					Metric: Metric{"percentile": "0.010000", MetricNameLabel: "rpc_latency_microseconds", "service": "bar", "job": "batch_exporter"},
					Value:  78.48563317257356,
				},
				&Sample{
					Metric: Metric{"percentile": "0.010000", MetricNameLabel: "rpc_latency_microseconds", "service": "foo", "job": "batch_exporter"},
					Value:  15.890724674774395,
				},
				&Sample{

					Metric: Metric{"percentile": "0.050000", MetricNameLabel: "rpc_latency_microseconds", "service": "zed", "job": "batch_exporter"},
					Value:  0.0459814091918713,
				},
				&Sample{
					Metric: Metric{"percentile": "0.050000", MetricNameLabel: "rpc_latency_microseconds", "service": "bar", "job": "batch_exporter"},
					Value:  78.48563317257356,
				},
				&Sample{

					Metric: Metric{"percentile": "0.050000", MetricNameLabel: "rpc_latency_microseconds", "service": "foo", "job": "batch_exporter"},
					Value:  15.890724674774395,
				},
				&Sample{
					Metric: Metric{"percentile": "0.500000", MetricNameLabel: "rpc_latency_microseconds", "service": "zed", "job": "batch_exporter"},
					Value:  0.6120456642749681,
				},
				&Sample{

					Metric: Metric{"percentile": "0.500000", MetricNameLabel: "rpc_latency_microseconds", "service": "bar", "job": "batch_exporter"},
					Value:  97.31798360385088,
				},
				&Sample{
					Metric: Metric{"percentile": "0.500000", MetricNameLabel: "rpc_latency_microseconds", "service": "foo", "job": "batch_exporter"},
					Value:  84.63044031436561,
				},
				&Sample{

					Metric: Metric{"percentile": "0.900000", MetricNameLabel: "rpc_latency_microseconds", "service": "zed", "job": "batch_exporter"},
					Value:  1.355915069887731,
				},
				&Sample{
					Metric: Metric{"percentile": "0.900000", MetricNameLabel: "rpc_latency_microseconds", "service": "bar", "job": "batch_exporter"},
					Value:  109.89202084295582,
				},
				&Sample{
					Metric: Metric{"percentile": "0.900000", MetricNameLabel: "rpc_latency_microseconds", "service": "foo", "job": "batch_exporter"},
					Value:  160.21100853053224,
				},
				&Sample{
					Metric: Metric{"percentile": "0.990000", MetricNameLabel: "rpc_latency_microseconds", "service": "zed", "job": "batch_exporter"},
					Value:  1.772733213161236,
				},
				&Sample{

					Metric: Metric{"percentile": "0.990000", MetricNameLabel: "rpc_latency_microseconds", "service": "bar", "job": "batch_exporter"},
					Value:  109.99626121011262,
				},
				&Sample{
					Metric: Metric{"percentile": "0.990000", MetricNameLabel: "rpc_latency_microseconds", "service": "foo", "job": "batch_exporter"},
					Value:  172.49828748957728,
				},
			},
		},
	}

	for i, scenario := range scenarios {
		inputChannel := make(chan *Result, 1024)

		defer close(inputChannel)

		reader, err := os.Open(path.Join("fixtures", scenario.in))
		if err != nil {
			t.Fatalf("%d. couldn't open scenario input file %s: %s", i, scenario.in, err)
		}

		options := &ProcessOptions{
			Timestamp:  time.Now(),
			BaseLabels: scenario.baseLabels,
		}
		err = Processor002.ProcessSingle(reader, inputChannel, options)
		if !errorEqual(scenario.err, err) {
			t.Errorf("%d. expected err of %s, got %s", i, scenario.err, err)
			continue
		}

		delivered := Samples{}

		for len(inputChannel) != 0 {
			result := <-inputChannel
			if result.Err != nil {
				t.Fatalf("%d. expected no error, got: %s", i, result.Err)
			}
			delivered = append(delivered, result.Samples...)
		}

		if len(delivered) != len(scenario.out) {
			t.Errorf("%d. expected output length of %d, got %d", i, len(scenario.out), len(delivered))

			continue
		}

		expectedElements := list.New()
		for _, j := range scenario.out {
			expectedElements.PushBack(j)
		}

		for j := 0; j < len(delivered); j++ {
			actual := delivered[j]

			found := false
			for element := expectedElements.Front(); element != nil && found == false; element = element.Next() {
				candidate := element.Value.(*Sample)

				if candidate.Value != actual.Value {
					continue
				}

				if len(candidate.Metric) != len(actual.Metric) {
					continue
				}

				labelsMatch := false

				for key, value := range candidate.Metric {
					actualValue, ok := actual.Metric[key]
					if !ok {
						break
					}
					if actualValue == value {
						labelsMatch = true
						break
					}
				}

				if !labelsMatch {
					continue
				}

				// XXX: Test time.
				found = true
				expectedElements.Remove(element)
			}

			if !found {
				t.Errorf("%d.%d. expected to find %s among candidate, absent", i, j, actual)
			}
		}
	}
}

func TestProcessor002Process(t *testing.T) {
	testProcessor002Process(t)
}

func BenchmarkProcessor002Process(b *testing.B) {
	b.StopTimer()

	pre := runtime.MemStats{}
	runtime.ReadMemStats(&pre)

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		testProcessor002Process(b)
	}

	post := runtime.MemStats{}
	runtime.ReadMemStats(&post)

	allocated := post.TotalAlloc - pre.TotalAlloc

	b.Logf("Allocated %d at %f per cycle with %d cycles.", allocated, float64(allocated)/float64(b.N), b.N)
}
