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

// MustNewUntyped is a version of NewUntyped that panics where NewUntyped would
// have returned an error.
func MustNewUntyped(desc *Desc) Untyped {
	u, err := NewUntyped(desc)
	if err != nil {
		panic(err)
	}
	return u
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

// MustNewUntypedVec is a version of NewUntypedVec that panics where NewUntypedVec would
// have returned an error.
func MustNewUntypedVec(desc *Desc) *UntypedVec {
	u, err := NewUntypedVec(desc)
	if err != nil {
		panic(err)
	}
	return u
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
