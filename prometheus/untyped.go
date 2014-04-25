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

	"github.com/prometheus/client_golang/model"
)

// An Untyped metric represents scalar values without any type implications
// whatsoever. If you need to handle values that cannot be represented by any of
// the existing metric types, you can use an Untyped type and rely on contracts
// outside of Prometheus to ensure that these values are understood correctly.
type Untyped interface {
	Metric
	Set(labels map[string]string, value float64) float64
}

type untypedVector struct {
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

// NewUntyped returns a newly allocated Untyped metric ready to be used.
func NewUntyped() Untyped {
	return &untyped{
		values: map[uint64]*untypedVector{},
	}
}

type untyped struct {
	mutex  sync.RWMutex
	values map[uint64]*untypedVector
}

func (metric *untyped) String() string {
	formatString := "[Untyped %s]"

	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	return fmt.Sprintf(formatString, metric.values)
}

func (metric *untyped) Set(labels map[string]string, value float64) float64 {
	if labels == nil {
		labels = blankLabelsSingleton
	}

	signature := model.LabelValuesToSignature(labels)

	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	if original, ok := metric.values[signature]; ok {
		original.Value = value
	} else {
		metric.values[signature] = &untypedVector{
			Labels: labels,
			Value:  value,
		}
	}

	return value
}

func (metric *untyped) Reset(labels map[string]string) {
	signature := model.LabelValuesToSignature(labels)

	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	delete(metric.values, signature)
}

func (metric *untyped) ResetAll() {
	metric.mutex.Lock()
	defer metric.mutex.Unlock()

	for key, value := range metric.values {
		for label := range value.Labels {
			delete(value.Labels, label)
		}
		delete(metric.values, key)
	}
}

func (metric *untyped) MarshalJSON() ([]byte, error) {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	values := make([]*untypedVector, 0, len(metric.values))
	for _, value := range metric.values {
		values = append(values, value)
	}

	return json.Marshal(map[string]interface{}{
		typeKey:  untypedTypeValue,
		valueKey: values,
	})
}

func (metric *untyped) dumpChildren(f *dto.MetricFamily) {
	metric.mutex.RLock()
	defer metric.mutex.RUnlock()

	f.Type = dto.MetricType_UNTYPED.Enum()

	for _, child := range metric.values {
		c := &dto.Untyped{
			Value: proto.Float64(child.Value),
		}

		m := &dto.Metric{
			Untyped: c,
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
