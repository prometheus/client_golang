// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"bytes"
	"fmt"
	"hash"
	"sort"
	"sync"

	dto "github.com/prometheus/client_model/go"
)

// MetricVec is a MetricsCollector to bundle metrics of the same name that
// differ in their label values. MetricVec is usually not used directly but as a
// building block for implementations of vectors of a given metric
// type. GaugeVec, CounterVec, SummaryVec, and UntypedVec are examples already
// provided with this library.
type MetricVec struct {
	mtx      sync.RWMutex
	children map[uint64]Metric
	desc     *Desc

	hash hash.Hash64
	buf  bytes.Buffer

	opts *SummaryOptions // Only needed for summaries.
}

// DescribeMetrics implements MetricsCollector. The length of the returned slice
// is always one.
func (m *MetricVec) DescribeMetrics() []*Desc {
	return []*Desc{m.desc}
}

// CollectMetrics implements MetricsCollector. It returns the metrics in hash
// order.
func (m *MetricVec) CollectMetrics() []Metric {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	hashes := make([]uint64, 0, len(m.children))
	metrics := make([]Metric, 0, len(m.children))
	for h := range m.children {
		hashes = append(hashes, h)
	}
	sort.Sort(hashSorter(hashes))
	for _, h := range hashes {
		metrics = append(metrics, m.children[h])
	}
	return metrics
}

// GetMetricWithLabelValues returns the metric where the variable lables have
// the values passed in as dims. The order must be the same as in the
// descriptor. If too many or too few arguments are usen, an error is returned.
func (m *MetricVec) GetMetricWithLabelValues(dims ...string) (Metric, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	h, err := m.hashLabelValues(dims)
	if err != nil {
		return nil, err
	}
	return m.getOrCreateMetric(h, dims...), nil
}

// GetMetricWithLabels returns the metric where the variable labels are the same
// as those passed in as labels. If the labels map has too many or too few
// entries, or if a name of a variable label cannot be found in the labels map,
// an error is returned.
func (m *MetricVec) GetMetricWithLabels(labels map[string]string) (Metric, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	h, err := m.hashLabels(labels)
	if err != nil {
		return nil, err
	}
	dims := make([]string, len(labels))
	for i, label := range m.desc.VariableLabels {
		dims[i] = labels[label]
	}
	return m.getOrCreateMetric(h, dims...), nil
}

// WithLabelValues works as GetMetricWithLabelValues, but panics if an error
// occurs. The method allows neat syntax like:
//   httpReqs.WithLabelValues("404", "POST").Inc()
func (m *MetricVec) WithLabelValues(dims ...string) Metric {
	metric, err := m.GetMetricWithLabelValues(dims...)
	if err != nil {
		panic(err)
	}
	return metric
}

// WithLabels works as GetMetricWithLabels, but panics if an error occurs. The
// method allows neat syntax like:
//   httpReqs.WithLabels(map[string]string{"status":"404", "method":"POST"}).Inc()
func (m *MetricVec) WithLabels(labels map[string]string) Metric {
	metric, err := m.GetMetricWithLabels(labels)
	if err != nil {
		panic(err)
	}
	return metric
}

// DeleteLabelValues removes the metric where the variable labels are the same
// as those passed in as labels. It returns true, if a metric was deleted.
func (m *MetricVec) DeleteLabelValues(dims ...string) bool {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	h, err := m.hashLabelValues(dims)
	if err != nil {
		return false
	}
	if _, has := m.children[h]; !has {
		return false
	}
	delete(m.children, h)
	return true
}

// DeleteLabels deletes the metric where the variable labels are the same
// as those passed in as labels. It returns true, if a metric was deleted.
func (m *MetricVec) DeleteLabels(labels map[string]string) bool {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	h, err := m.hashLabels(labels)
	if err != nil {
		return false
	}
	if _, has := m.children[h]; !has {
		return false
	}
	delete(m.children, h)
	return true
}

func (m *MetricVec) hashLabelValues(vals []string) (uint64, error) {
	if len(vals) != len(m.desc.VariableLabels) {
		return 0, errInconsistentCardinality
	}
	m.hash.Reset()
	for _, val := range vals {
		m.buf.Reset()
		m.buf.WriteString(val)
		m.hash.Write(m.buf.Bytes())
	}
	return m.hash.Sum64(), nil
}

func (m *MetricVec) hashLabels(labels map[string]string) (uint64, error) {
	if len(labels) != len(m.desc.VariableLabels) {
		return 0, errInconsistentCardinality
	}
	m.hash.Reset()
	for _, label := range m.desc.VariableLabels {
		val, ok := labels[label]
		if !ok {
			return 0, fmt.Errorf("label name %q missing in label map", label)
		}
		m.buf.Reset()
		m.buf.WriteString(val)
		m.hash.Write(m.buf.Bytes())
	}
	return m.hash.Sum64(), nil
}

func (m *MetricVec) getOrCreateMetric(hash uint64, dims ...string) Metric {
	var err error
	metric, ok := m.children[hash]
	if !ok {
		// Copy dims so they don't have to be allocated even if we don't go
		// down this code path.
		copiedDims := append(make([]string, 0, len(dims)), dims...)
		if m.desc.Type == dto.MetricType_SUMMARY {
			if metric, err = newSummary(m.desc, m.opts, copiedDims...); err != nil {
				panic(err) // Cannot happen.
			}
		} else {
			if metric, err = NewValue(m.desc, 0, copiedDims...); err != nil {
				panic(err) // Cannot happen.
			}
		}
		m.children[hash] = metric
	}
	return metric
}
