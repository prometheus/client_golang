// Copyright 2014 Prometheus Team
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

package prometheus

import (
	"errors"
	"hash/fnv"

	dto "github.com/prometheus/client_model/go"
)

var (
	errCannotDecreaseCounter = errors.New("counter cannot decrease in value")
)

type Counter interface {
	Metric
	MetricsCollector

	Set(float64)
	Inc()
	// Add panics if float64 < 0.
	Add(float64)
}

// NewCounter creates a new counter (without labels) based on the provided
// descriptor. The Type field in the descriptor is ignored and forcefully set to
// MetricType_COUNTER.
func NewCounter(desc *Desc) (Counter, error) {
	if len(desc.VariableLabels) > 0 {
		return nil, errLabelsForSimpleMetric
	}
	desc.Type = dto.MetricType_COUNTER
	result := &counter{Value: Value{desc: desc}}
	result.MetricSlice = []Metric{result}
	result.DescSlice = []*Desc{desc}
	return result, nil
}

// MustNewCounter is a version of NewCounter that panics where NewCounter would
// have returned an error.
func MustNewCounter(desc *Desc) Counter {
	c, err := NewCounter(desc)
	if err != nil {
		panic(err)
	}
	return c
}

type counter struct {
	Value
}

func (c *counter) Add(v float64) {
	if v < 0 {
		panic(errCannotDecreaseCounter)
	}
	c.Value.Add(v)
}

type CounterVec struct {
	MetricVec
}

func NewCounterVec(desc *Desc) (*CounterVec, error) {
	if len(desc.VariableLabels) == 0 {
		return nil, errNoLabelsForVecMetric
	}
	desc.Type = dto.MetricType_COUNTER
	return &CounterVec{
		MetricVec: MetricVec{
			children: map[uint64]Metric{},
			desc:     desc,
			hash:     fnv.New64a(),
		},
	}, nil
}

// MustNewCounterVec is a version of NewCounterVec that panics where NewCounterVec would
// have returned an error.
func MustNewCounterVec(desc *Desc) *CounterVec {
	c, err := NewCounterVec(desc)
	if err != nil {
		panic(err)
	}
	return c
}

func (m *CounterVec) GetMetricWithLabelValues(dims ...string) (Counter, error) {
	metric, err := m.MetricVec.GetMetricWithLabelValues(dims...)
	return metric.(Counter), err
}

func (m *CounterVec) GetMetricWithLabels(labels map[string]string) (Counter, error) {
	metric, err := m.MetricVec.GetMetricWithLabels(labels)
	return metric.(Counter), err
}

func (m *CounterVec) WithLabelValues(dims ...string) Counter {
	return m.MetricVec.WithLabelValues(dims...).(Counter)
}

func (m *CounterVec) WithLabels(labels map[string]string) Counter {
	return m.MetricVec.WithLabels(labels).(Counter)
}
