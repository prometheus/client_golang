// Copyright 2018 The Prometheus Authors
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

package internal

import (
	"cmp"
	"slices"

	dto "github.com/prometheus/client_model/go"
)

// LabelPairSorter implements sort.Interface. It is used to sort a slice of
// dto.LabelPair pointers.
type LabelPairSorter []*dto.LabelPair

func (s LabelPairSorter) Len() int {
	return len(s)
}

func (s LabelPairSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s LabelPairSorter) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

// MetricSorter is a sortable slice of *dto.Metric.
type MetricSorter []*dto.Metric

func (s MetricSorter) Len() int {
	return len(s)
}

func (s MetricSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s MetricSorter) Less(i, j int) bool {
	return compareMetrics(s[i], s[j]) < 0
}

func compareMetrics(a, b *dto.Metric) int {
	if c := cmp.Compare(len(a.Label), len(b.Label)); c != 0 {
		return c
	}
	for n, lp := range a.Label {
		if c := cmp.Compare(lp.GetValue(), b.Label[n].GetValue()); c != 0 {
			return c
		}
	}

	if a.TimestampMs == nil && b.TimestampMs == nil {
		return 0
	}
	if a.TimestampMs == nil {
		return 1
	}
	if b.TimestampMs == nil {
		return -1
	}
	return cmp.Compare(a.GetTimestampMs(), b.GetTimestampMs())
}

// NormalizeMetricFamilies returns a MetricFamily slice with empty
// MetricFamilies pruned and the remaining MetricFamilies sorted by name within
// the slice, with the contained Metrics sorted within each MetricFamily.
func NormalizeMetricFamilies(metricFamiliesByName map[string]*dto.MetricFamily) []*dto.MetricFamily {
	for _, mf := range metricFamiliesByName {
		slices.SortFunc(mf.Metric, compareMetrics)
	}
	names := make([]string, 0, len(metricFamiliesByName))
	for name, mf := range metricFamiliesByName {
		if len(mf.Metric) > 0 {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	result := make([]*dto.MetricFamily, 0, len(names))
	for _, name := range names {
		result = append(result, metricFamiliesByName[name])
	}
	return result
}
