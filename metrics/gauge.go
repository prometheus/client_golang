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

/*
A gauge metric merely provides an instantaneous representation of a scalar
value or an accumulation.  For instance, if one wants to expose the current
temperature or the hitherto bandwidth used, this would be the metric for such
circumstances.
*/
type GaugeMetric struct {
	value float64
	mutex sync.RWMutex
}

func (metric *GaugeMetric) String() string {
	formatString := "[GaugeMetric; value=%f]"

	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	return fmt.Sprintf(formatString, metric.value)
}

func (metric *GaugeMetric) Set(value float64) float64 {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	metric.value = value

	return metric.value
}

func (metric *GaugeMetric) Get() float64 {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	return metric.value
}

func (metric *GaugeMetric) Marshallable() map[string]interface{} {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	v := make(map[string]interface{}, 2)

	v[valueKey] = metric.value
	v[typeKey] = gaugeTypeValue

	return v
}
