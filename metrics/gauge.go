/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package metrics

import (
	"fmt"
	"github.com/matttproud/golang_instrumentation/utility"
	"sync"
)

/*
A gauge metric merely provides an instantaneous representation of a scalar
value or an accumulation.  For instance, if one wants to expose the current
temperature or the hitherto bandwidth used, this would be the metric for such
circumstances.
*/
type Gauge interface {
	AsMarshallable() map[string]interface{}
	ResetAll()
	Set(labels map[string]string, value float64) float64
	String() string
}

type gaugeValue struct {
	labels map[string]string
	value  float64
}

func NewGauge() Gauge {
	return &gauge{
		values: map[string]*gaugeValue{},
	}
}

type gauge struct {
	mutex  sync.RWMutex
	values map[string]*gaugeValue
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

	signature := utility.LabelsToSignature(labels)

	if original, ok := metric.values[signature]; ok {
		original.value = value
	} else {
		metric.values[signature] = &gaugeValue{
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

func (metric *gauge) AsMarshallable() map[string]interface{} {
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
