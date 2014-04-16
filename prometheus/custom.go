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

// A Custom metric represents scalar values without any type implications
// whatsoever. If you need to handle values that cannot be represented by any of
// the existing metric types, you can use a Custom type and rely on contracts
// outside of Prometheus to ensure that these values are understood correctly.
type Custom interface {
	Metric
	Set(labels map[string]string, value float64) float64
}

type customVector struct {
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

// NewCustom returns a newly allocated Custom metric ready to be used.
func NewCustom() Custom {
	return &custom{
		values: map[uint64]*customVector{},
	}
}

type custom struct {
	mutex  sync.RWMutex
	values map[uint64]*customVector
}

func (metric *custom) String() string {
	formatString := "[Custom %s]"

	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	return fmt.Sprintf(formatString, metric.values)
}

func (metric *custom) Set(labels map[string]string, value float64) float64 {
	if labels == nil {
		labels = blankLabelsSingleton
	}

	signature := labelValuesToSignature(labels)

	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	if original, ok := metric.values[signature]; ok {
		original.Value = value
	} else {
		metric.values[signature] = &customVector{
			Labels: labels,
			Value:  value,
		}
	}

	return value
}

func (metric *custom) Reset(labels map[string]string) {
	signature := labelValuesToSignature(labels)

	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	delete(metric.values, signature)
}

func (metric *custom) ResetAll() {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	for key, value := range metric.values {
		for label := range value.Labels {
			delete(value.Labels, label)
		}
		delete(metric.values, key)
	}
}

func (metric *custom) MarshalJSON() ([]byte, error) {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	values := make([]*customVector, 0, len(metric.values))
	for _, value := range metric.values {
		values = append(values, value)
	}

	return json.Marshal(map[string]interface{}{
		typeKey:  customTypeValue,
		valueKey: values,
	})
}

func (metric *custom) dumpChildren(f *dto.MetricFamily) {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	f.Type = dto.MetricType_CUSTOM.Enum()

	for _, child := range metric.values {
		c := &dto.Custom{
			Value: proto.Float64(child.Value),
		}

		m := &dto.Metric{
			Custom: c,
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
