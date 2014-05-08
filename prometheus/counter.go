// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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

// NewCounter emits a new Counter from the provided descriptor.
// The Type field is ignored and forcefully set to MetricType_COUNTER.
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
