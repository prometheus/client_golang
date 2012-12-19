/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package metrics

import (
	"fmt"
	"sync"
)

type CounterMetric struct {
	value float64
	mutex sync.RWMutex
}

func (metric *CounterMetric) Set(value float64) float64 {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	metric.value = value

	return metric.value
}

func (metric *CounterMetric) Reset() {
	metric.Set(0)
}

func (metric *CounterMetric) String() string {
	formatString := "[CounterMetric; value=%f]"

	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	return fmt.Sprintf(formatString, metric.value)
}

func (metric *CounterMetric) IncrementBy(value float64) float64 {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	metric.value += value

	return metric.value
}

func (metric *CounterMetric) Increment() float64 {
	return metric.IncrementBy(1)
}

func (metric *CounterMetric) DecrementBy(value float64) float64 {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	metric.value -= value

	return metric.value
}

func (metric *CounterMetric) Decrement() float64 {
	return metric.DecrementBy(1)
}

func (metric *CounterMetric) Get() float64 {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	return metric.value
}

func (metric *CounterMetric) Marshallable() map[string]interface{} {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	v := make(map[string]interface{}, 2)

	v[valueKey] = metric.value
	v[typeKey] = counterTypeValue

	return v
}
