// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"hash/fnv"

	dto "github.com/prometheus/client_model/go"
)

// Untyped proxies an untyped scalar value.
type Untyped interface {
	Metric
	MetricsCollector

	Set(float64)
	Inc()
	Dec()
	Add(float64)
	Sub(float64)
}

// NewUntyped emits a new Untyped metric from the provided descriptor.
// The descriptor's Type field is ignored and forcefully set to MetricType_UNTYPED.
func NewUntyped(desc *Desc) (Untyped, error) {
	if len(desc.VariableLabels) > 0 {
		return nil, errLabelsForSimpleMetric
	}
	desc.Type = dto.MetricType_UNTYPED
	return NewValue(desc, 0)
}

type UntypedVec struct {
	MetricVec
}

func NewUntypedVec(desc *Desc) (*UntypedVec, error) {
	if len(desc.VariableLabels) == 0 {
		return nil, errNoLabelsForVecMetric
	}
	desc.Type = dto.MetricType_UNTYPED
	return &UntypedVec{
		MetricVec: MetricVec{
			children: map[uint64]Metric{},
			desc:     desc,
			hash:     fnv.New64a(),
		},
	}, nil
}

func (m *UntypedVec) GetMetricWithLabelValues(dims ...string) (Untyped, error) {
	metric, err := m.MetricVec.GetMetricWithLabelValues(dims...)
	return metric.(Untyped), err
}

func (m *UntypedVec) GetMetricWithLabels(labels map[string]string) (Untyped, error) {
	metric, err := m.MetricVec.GetMetricWithLabels(labels)
	return metric.(Untyped), err
}

func (m *UntypedVec) WithLabelValues(dims ...string) Untyped {
	return m.MetricVec.WithLabelValues(dims...).(Untyped)
}

func (m *UntypedVec) WithLabels(labels map[string]string) Untyped {
	return m.MetricVec.WithLabels(labels).(Untyped)
}
