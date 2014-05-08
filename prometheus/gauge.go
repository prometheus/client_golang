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

// Gauge proxies a scalar value.
type Gauge interface {
	Metric
	MetricsCollector

	Set(float64)
	Inc()
	Dec()
	Add(float64)
	Sub(float64)
}

// NewGauge emits a new Gauge from the provided descriptor.
// The descriptor's Type field is ignored and forcefully set to MetricType_GAUGE.
func NewGauge(desc *Desc) (Gauge, error) {
	if len(desc.VariableLabels) > 0 {
		return nil, errLabelsForSimpleMetric
	}
	desc.Type = dto.MetricType_GAUGE
	return NewValue(desc, 0)
}

// MustNewGauge is a version of NewGauge that panics where NewGauge would
// have returned an error.
func MustNewGauge(desc *Desc) Gauge {
	g, err := NewGauge(desc)
	if err != nil {
		panic(err)
	}
	return g
}

type GaugeVec struct {
	MetricVec
}

func NewGaugeVec(desc *Desc) (*GaugeVec, error) {
	if len(desc.VariableLabels) == 0 {
		return nil, errNoLabelsForVecMetric
	}
	desc.Type = dto.MetricType_GAUGE
	return &GaugeVec{
		MetricVec: MetricVec{
			children: map[uint64]Metric{},
			desc:     desc,
			hash:     fnv.New64a(),
		},
	}, nil
}

// MustNewGaugeVec is a version of NewGaugeVec that panics where NewGaugeVec would
// have returned an error.
func MustNewGaugeVec(desc *Desc) *GaugeVec {
	g, err := NewGaugeVec(desc)
	if err != nil {
		panic(err)
	}
	return g
}

func (m *GaugeVec) GetMetricWithLabelValues(dims ...string) (Gauge, error) {
	metric, err := m.MetricVec.GetMetricWithLabelValues(dims...)
	return metric.(Gauge), err
}

func (m *GaugeVec) GetMetricWithLabels(labels map[string]string) (Gauge, error) {
	metric, err := m.MetricVec.GetMetricWithLabels(labels)
	return metric.(Gauge), err
}

func (m *GaugeVec) WithLabelValues(dims ...string) Gauge {
	return m.MetricVec.WithLabelValues(dims...).(Gauge)
}

func (m *GaugeVec) WithLabels(labels map[string]string) Gauge {
	return m.MetricVec.WithLabels(labels).(Gauge)
}
