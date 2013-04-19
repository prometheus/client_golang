// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"fmt"
	"sync"
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
	labels map[string]string
	value  float64
}

func NewGauge() Gauge {
	return &gauge{
		values: map[string]*gaugeVector{},
	}
}

type gauge struct {
	mutex  sync.RWMutex
	values map[string]*gaugeVector
}

func (metric gauge) String() string {
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
		original.value = value
	} else {
		metric.values[signature] = &gaugeVector{
			labels: labels,
			value:  value,
		}
	}

	return value
}

func (metric *gauge) ResetAll() {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	for key, value := range metric.values {
		for label := range value.labels {
			delete(value.labels, label)
		}
		delete(metric.values, key)
	}
}

func (metric gauge) AsMarshallable() map[string]interface{} {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	values := make([]map[string]interface{}, 0, len(metric.values))
	for _, value := range metric.values {
		values = append(values, map[string]interface{}{
			labelsKey: value.labels,
			valueKey:  value.value,
		})
	}

	return map[string]interface{}{
		typeKey:  gaugeTypeValue,
		valueKey: values,
	}
}
