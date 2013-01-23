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

// TODO(matt): Refactor to de-duplicate behaviors.

type Counter interface {
	AsMarshallable() map[string]interface{}
	Decrement(labels map[string]string) float64
	DecrementBy(labels map[string]string, value float64) float64
	Increment(labels map[string]string) float64
	IncrementBy(labels map[string]string, value float64) float64
	ResetAll()
	Set(labels map[string]string, value float64) float64
	String() string
}

type counterValue struct {
	labels map[string]string
	value  float64
}

func NewCounter() Counter {
	return &counter{
		values: map[string]*counterValue{},
	}
}

type counter struct {
	mutex  sync.RWMutex
	values map[string]*counterValue
}

func (metric *counter) Set(labels map[string]string, value float64) float64 {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	if labels == nil {
		labels = map[string]string{}
	}

	signature := utility.LabelsToSignature(labels)
	if original, ok := metric.values[signature]; ok {
		original.value = value
	} else {
		metric.values[signature] = &counterValue{
			labels: labels,
			value:  value,
		}
	}

	return value
}

func (metric *counter) ResetAll() {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	for key, value := range metric.values {
		for label := range value.labels {
			delete(value.labels, label)
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

	signature := utility.LabelsToSignature(labels)
	if original, ok := metric.values[signature]; ok {
		original.value += value
	} else {
		metric.values[signature] = &counterValue{
			labels: labels,
			value:  value,
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

	signature := utility.LabelsToSignature(labels)
	if original, ok := metric.values[signature]; ok {
		original.value -= value
	} else {
		metric.values[signature] = &counterValue{
			labels: labels,
			value:  -1 * value,
		}
	}

	return value
}

func (metric *counter) Decrement(labels map[string]string) float64 {
	return metric.DecrementBy(labels, 1)
}

func (metric *counter) AsMarshallable() map[string]interface{} {
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
		valueKey: values,
		typeKey:  counterTypeValue,
	}
}
