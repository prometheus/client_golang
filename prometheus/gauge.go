// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"encoding/json"
	"fmt"
	"sync"

	"code.google.com/p/goprotobuf/proto"

	dto "github.com/prometheus/client_model/go"
)

// A gauge metric merely provides an instantaneous representation of a scalar
// value or an accumulation.  For instance, if one wants to expose the current
// temperature or the hitherto bandwidth used, this would be the metric for such
// circumstances.
type Gauge interface {
	Metric
	Set(labels map[string]string, value float64) float64
}

type gaugeVector struct {
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

func NewGauge() Gauge {
	return &gauge{
		values: map[uint64]*gaugeVector{},
	}
}

type gauge struct {
	mutex  sync.RWMutex
	values map[uint64]*gaugeVector
}

func (metric *gauge) String() string {
	formatString := "[Gauge %s]"

	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	return fmt.Sprintf(formatString, metric.values)
}

func (metric *gauge) Set(labels map[string]string, value float64) float64 {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	if labels == nil {
		labels = map[string]string{}
	}

	signature := labelsToSignature(labels)

	if original, ok := metric.values[signature]; ok {
		original.Value = value
	} else {
		metric.values[signature] = &gaugeVector{
			Labels: labels,
			Value:  value,
		}
	}

	return value
}

func (metric *gauge) ResetAll() {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	for key, value := range metric.values {
		for label := range value.Labels {
			delete(value.Labels, label)
		}
		delete(metric.values, key)
	}
}

func (metric *gauge) MarshalJSON() ([]byte, error) {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	values := make([]*gaugeVector, 0, len(metric.values))
	for _, value := range metric.values {
		values = append(values, value)
	}

	return json.Marshal(map[string]interface{}{
		typeKey:  gaugeTypeValue,
		valueKey: values,
	})
}

func (metric *gauge) dumpChildren(f *dto.MetricFamily) {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	f.Type = dto.MetricType_GAUGE.Enum()

	for _, child := range metric.values {
		c := &dto.Gauge{
			Value: proto.Float64(child.Value),
		}

		m := &dto.Metric{
			Gauge: c,
		}

		for name, value := range child.Labels {
			p := &dto.LabelPair{
				Name:  proto.String(name),
				Value: proto.String(value),
			}

			m.Label = append(m.Label, p)
		}

		f.Metric = append(f.Metric, m)
	}
}
