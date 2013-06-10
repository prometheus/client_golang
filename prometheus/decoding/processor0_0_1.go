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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

const (
	baseLabels001 = "baseLabels"
	counter001    = "counter"
	docstring001  = "docstring"
	gauge001      = "gauge"
	histogram001  = "histogram"
	labels001     = "labels"
	metric001     = "metric"
	type001       = "type"
	value001      = "value"
	percentile001 = "percentile"
)

// Processor002 is responsible for decoding payloads from protocol version
// 0.0.1.
var Processor001 Processor = &processor001{}

// processor001 is responsible for handling API version 0.0.1.
type processor001 struct {
	time Time
}

// entity001 represents a the JSON structure that 0.0.1 uses.
type entity001 []struct {
	BaseLabels map[string]string `json:"baseLabels"`
	Docstring  string            `json:"docstring"`
	Metric     struct {
		MetricType string `json:"type"`
		Value      []struct {
			Labels map[string]string `json:"labels"`
			Value  interface{}       `json:"value"`
		} `json:"value"`
	} `json:"metric"`
}

func (p *processor001) ProcessSingle(in io.Reader, out chan<- *Result, o *ProcessOptions) error {
	// TODO(matt): Replace with plain-jane JSON unmarshalling.
	buffer, err := ioutil.ReadAll(in)
	if err != nil {
		return err
	}

	entities := entity001{}
	if err = json.Unmarshal(buffer, &entities); err != nil {
		return err
	}

	// TODO(matt): This outer loop is a great basis for parallelization.
	pendingSamples := Samples{}
	for _, entity := range entities {
		for _, value := range entity.Metric.Value {
			entityLabels := labelSet(entity.BaseLabels).Merge(labelSet(value.Labels))
			labels := mergeTargetLabels(entityLabels, o.BaseLabels)

			switch entity.Metric.MetricType {
			case gauge001, counter001:
				sampleValue, ok := value.Value.(float64)
				if !ok {
					err = fmt.Errorf("Could not convert value from %s %s to float64.", entity, value)
					out <- &Result{Err: err}
					continue
				}

				pendingSamples = append(pendingSamples, &Sample{
					Metric:    Metric(labels),
					Timestamp: o.Timestamp,
					Value:     SampleValue(sampleValue),
				})

				break

			case histogram001:
				sampleValue, ok := value.Value.(map[string]interface{})
				if !ok {
					err = fmt.Errorf("Could not convert value from %q to a map[string]interface{}.", value.Value)
					out <- &Result{Err: err}
					continue
				}

				for percentile, percentileValue := range sampleValue {
					individualValue, ok := percentileValue.(float64)
					if !ok {
						err = fmt.Errorf("Could not convert value from %q to a float64.", percentileValue)
						out <- &Result{Err: err}
						continue
					}

					childMetric := make(map[LabelName]LabelValue, len(labels)+1)

					for k, v := range labels {
						childMetric[k] = v
					}

					childMetric[LabelName(percentile001)] = LabelValue(percentile)

					pendingSamples = append(pendingSamples, &Sample{
						Metric:    Metric(childMetric),
						Timestamp: o.Timestamp,
						Value:     SampleValue(individualValue),
					})
				}

				break
			}
		}
	}
	if len(pendingSamples) > 0 {
		out <- &Result{Samples: pendingSamples}
	}

	return nil
}
