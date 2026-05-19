// Copyright 2014 The Prometheus Authors
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
	"sync"
	"sync/atomic"
	"time"
)

// ttlMetricWithLabelValues is the TTL variant of metricWithLabelValues.
type ttlMetricWithLabelValues struct {
	values       []string
	metric       Metric
	lastAccessed atomic.Int64 // unix timestamp in milliseconds
}

// ttlMetricMap backs MetricVec instances created with NewMetricVecWithTTL.
// It is separate from metricMap so the default Vec path stays unchanged.
type ttlMetricMap struct {
	mtx       sync.RWMutex
	metrics   map[uint64][]*ttlMetricWithLabelValues
	desc      *Desc
	newMetric func(labelValues ...string) Metric
	ttl       time.Duration
}

func (m *ttlMetricMap) Describe(ch chan<- *Desc) {
	ch <- m.desc
}

func (m *ttlMetricMap) Collect(ch chan<- Metric) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	deadline := time.Now().Add(-m.ttl).UnixMilli()
	for _, metrics := range m.metrics {
		for i := range metrics {
			if metrics[i].lastAccessed.Load() < deadline {
				continue
			}
			ch <- metrics[i].metric
		}
	}
}

func (m *ttlMetricMap) Reset() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	for h := range m.metrics {
		delete(m.metrics, h)
	}
}

func (m *ttlMetricMap) touchByHash(h uint64, lvs []string, curry []curriedLabelValue) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	m.touchByHashRLocked(h, lvs, curry)
}

func (m *ttlMetricMap) touchByHashRLocked(h uint64, lvs []string, curry []curriedLabelValue) {
	now := time.Now().UnixMilli()
	metrics, ok := m.metrics[h]
	if !ok {
		return
	}
	if i := findTTLMetricWithLabelValues(metrics, lvs, curry); i < len(metrics) {
		metrics[i].lastAccessed.Store(now)
	}
}

func (m *ttlMetricMap) touchByHashLocked(h uint64, lvs []string, curry []curriedLabelValue) {
	now := time.Now().UnixMilli()
	metrics, ok := m.metrics[h]
	if !ok {
		return
	}
	if i := findTTLMetricWithLabelValues(metrics, lvs, curry); i < len(metrics) {
		metrics[i].lastAccessed.Store(now)
	}
}

func (m *ttlMetricMap) touchByHashLabels(h uint64, labels Labels, curry []curriedLabelValue) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	m.touchByHashLabelsRLocked(h, labels, curry)
}

func (m *ttlMetricMap) touchByHashLabelsRLocked(h uint64, labels Labels, curry []curriedLabelValue) {
	now := time.Now().UnixMilli()
	metrics, ok := m.metrics[h]
	if !ok {
		return
	}
	if i := findTTLMetricWithLabels(m.desc, metrics, labels, curry); i < len(metrics) {
		metrics[i].lastAccessed.Store(now)
	}
}

func (m *ttlMetricMap) touchByHashLabelsLocked(h uint64, labels Labels, curry []curriedLabelValue) {
	now := time.Now().UnixMilli()
	metrics, ok := m.metrics[h]
	if !ok {
		return
	}
	if i := findTTLMetricWithLabels(m.desc, metrics, labels, curry); i < len(metrics) {
		metrics[i].lastAccessed.Store(now)
	}
}

func (m *ttlMetricMap) cleanupExpired() int {
	deadline := time.Now().Add(-m.ttl).UnixMilli()
	m.mtx.Lock()
	defer m.mtx.Unlock()

	var numDeleted int
	for h, metrics := range m.metrics {
		origLen := len(metrics)
		remaining := metrics[:0]
		for i := range metrics {
			if metrics[i].lastAccessed.Load() >= deadline {
				remaining = append(remaining, metrics[i])
			} else {
				numDeleted++
			}
		}
		if len(remaining) == 0 {
			delete(m.metrics, h)
		} else {
			for i := len(remaining); i < origLen; i++ {
				metrics[i] = nil
			}
			m.metrics[h] = remaining
		}
	}
	return numDeleted
}

func (m *ttlMetricMap) deleteByHashWithLabelValues(
	h uint64, lvs []string, curry []curriedLabelValue,
) bool {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	metrics, ok := m.metrics[h]
	if !ok {
		return false
	}

	i := findTTLMetricWithLabelValues(metrics, lvs, curry)
	if i >= len(metrics) {
		return false
	}

	if len(metrics) > 1 {
		old := metrics
		m.metrics[h] = append(metrics[:i], metrics[i+1:]...)
		old[len(old)-1] = nil
	} else {
		delete(m.metrics, h)
	}
	return true
}

func (m *ttlMetricMap) deleteByHashWithLabels(
	h uint64, labels Labels, curry []curriedLabelValue,
) bool {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	metrics, ok := m.metrics[h]
	if !ok {
		return false
	}
	i := findTTLMetricWithLabels(m.desc, metrics, labels, curry)
	if i >= len(metrics) {
		return false
	}

	if len(metrics) > 1 {
		old := metrics
		m.metrics[h] = append(metrics[:i], metrics[i+1:]...)
		old[len(old)-1] = nil
	} else {
		delete(m.metrics, h)
	}
	return true
}

func (m *ttlMetricMap) deleteByLabels(labels Labels, curry []curriedLabelValue) int {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	var numDeleted int

	for h, metrics := range m.metrics {
		i := findTTLMetricWithPartialLabels(m.desc, metrics, labels, curry)
		if i >= len(metrics) {
			continue
		}
		delete(m.metrics, h)
		numDeleted++
	}

	return numDeleted
}

func findTTLMetricWithPartialLabels(
	desc *Desc, metrics []*ttlMetricWithLabelValues, labels Labels, curry []curriedLabelValue,
) int {
	for i := range metrics {
		if matchPartialLabels(desc, metrics[i].values, labels, curry) {
			return i
		}
	}
	return len(metrics)
}

func (m *ttlMetricMap) getOrCreateMetricWithLabelValues(
	hash uint64, lvs []string, curry []curriedLabelValue,
) Metric {
	m.mtx.RLock()
	metric, ok := m.getMetricWithHashAndLabelValues(hash, lvs, curry)
	m.mtx.RUnlock()
	if ok {
		m.touchByHash(hash, lvs, curry)
		return metric
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()
	metric, ok = m.getMetricWithHashAndLabelValues(hash, lvs, curry)
	if !ok {
		inlinedLVs := inlineLabelValues(lvs, curry)
		metric = m.newMetric(inlinedLVs...)
		entry := &ttlMetricWithLabelValues{values: inlinedLVs, metric: metric}
		entry.lastAccessed.Store(time.Now().UnixMilli())
		m.metrics[hash] = append(m.metrics[hash], entry)
	} else {
		m.touchByHashLocked(hash, lvs, curry)
	}
	return metric
}

func (m *ttlMetricMap) getOrCreateMetricWithLabels(
	hash uint64, labels Labels, curry []curriedLabelValue,
) Metric {
	m.mtx.RLock()
	metric, ok := m.getMetricWithHashAndLabels(hash, labels, curry)
	m.mtx.RUnlock()
	if ok {
		m.touchByHashLabels(hash, labels, curry)
		return metric
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()
	metric, ok = m.getMetricWithHashAndLabels(hash, labels, curry)
	if !ok {
		lvs := extractLabelValues(m.desc, labels, curry)
		metric = m.newMetric(lvs...)
		entry := &ttlMetricWithLabelValues{values: lvs, metric: metric}
		entry.lastAccessed.Store(time.Now().UnixMilli())
		m.metrics[hash] = append(m.metrics[hash], entry)
	} else {
		m.touchByHashLabelsLocked(hash, labels, curry)
	}
	return metric
}

func (m *ttlMetricMap) getMetricWithHashAndLabelValues(
	h uint64, lvs []string, curry []curriedLabelValue,
) (Metric, bool) {
	metrics, ok := m.metrics[h]
	if ok {
		if i := findTTLMetricWithLabelValues(metrics, lvs, curry); i < len(metrics) {
			return metrics[i].metric, true
		}
	}
	return nil, false
}

func (m *ttlMetricMap) getMetricWithHashAndLabels(
	h uint64, labels Labels, curry []curriedLabelValue,
) (Metric, bool) {
	metrics, ok := m.metrics[h]
	if ok {
		if i := findTTLMetricWithLabels(m.desc, metrics, labels, curry); i < len(metrics) {
			return metrics[i].metric, true
		}
	}
	return nil, false
}

func findTTLMetricWithLabelValues(
	metrics []*ttlMetricWithLabelValues, lvs []string, curry []curriedLabelValue,
) int {
	for i := range metrics {
		if matchLabelValues(metrics[i].values, lvs, curry) {
			return i
		}
	}
	return len(metrics)
}

func findTTLMetricWithLabels(
	desc *Desc, metrics []*ttlMetricWithLabelValues, labels Labels, curry []curriedLabelValue,
) int {
	for i := range metrics {
		if matchLabels(desc, metrics[i].values, labels, curry) {
			return i
		}
	}
	return len(metrics)
}
