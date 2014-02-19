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

	dto "github.com/prometheus/client_model/go"

	"code.google.com/p/goprotobuf/proto"
)

// TODO(matt): Refactor to de-duplicate behaviors.

type Counter interface {
	Metric

	Decrement(labels map[string]string) float64
	DecrementBy(labels map[string]string, value float64) float64
	Increment(labels map[string]string) float64
	IncrementBy(labels map[string]string, value float64) float64
	Set(labels map[string]string, value float64) float64
}

type counterVector struct {
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

func NewCounter() Counter {
	return &counter{
		values: map[uint64]*counterVector{},
	}
}

type counter struct {
	mutex  sync.RWMutex
	values map[uint64]*counterVector
}

func (metric *counter) Set(labels map[string]string, value float64) float64 {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	if labels == nil {
		labels = map[string]string{}
	}

	signature := labelsToSignature(labels)
	if original, ok := metric.values[signature]; ok {
		original.Value = value
	} else {
		metric.values[signature] = &counterVector{
			Labels: labels,
			Value:  value,
		}
	}

	return value
}

func (metric *counter) Reset(labels map[string]string) {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	signature := labelsToSignature(labels)
	delete(metric.values, signature)
}

func (metric *counter) ResetAll() {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	for key, value := range metric.values {
		for label := range value.Labels {
			delete(value.Labels, label)
		}
		delete(metric.values, key)
	}
}

func (metric *counter) String() string {
	formatString := "[Counter %s]"

	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	return fmt.Sprintf(formatString, metric.values)
}

func (metric *counter) IncrementBy(labels map[string]string, value float64) float64 {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	if labels == nil {
		labels = map[string]string{}
	}

	signature := labelsToSignature(labels)
	if original, ok := metric.values[signature]; ok {
		original.Value += value
	} else {
		metric.values[signature] = &counterVector{
			Labels: labels,
			Value:  value,
		}
	}

	return value
}

func (metric *counter) Increment(labels map[string]string) float64 {
	return metric.IncrementBy(labels, 1)
}

func (metric *counter) DecrementBy(labels map[string]string, value float64) float64 {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	if labels == nil {
		labels = map[string]string{}
	}

	signature := labelsToSignature(labels)
	if original, ok := metric.values[signature]; ok {
		original.Value -= value
	} else {
		metric.values[signature] = &counterVector{
			Labels: labels,
			Value:  -1 * value,
		}
	}

	return value
}

func (metric *counter) Decrement(labels map[string]string) float64 {
	return metric.DecrementBy(labels, 1)
}

func (metric *counter) MarshalJSON() ([]byte, error) {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	values := make([]*counterVector, 0, len(metric.values))

	for _, value := range metric.values {
		values = append(values, value)
	}

	return json.Marshal(map[string]interface{}{
		valueKey: values,
		typeKey:  counterTypeValue,
	})
}

func (metric *counter) dumpChildren(f *dto.MetricFamily) {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	f.Type = dto.MetricType_COUNTER.Enum()

	for _, child := range metric.values {
		c := &dto.Counter{
			Value: proto.Float64(child.Value),
		}

		m := &dto.Metric{
			Counter: c,
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
