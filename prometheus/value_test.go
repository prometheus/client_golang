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

package prometheus

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewConstMetricInvalidLabelValues(t *testing.T) {
	testCases := []struct {
		desc   string
		labels Labels
	}{
		{
			desc:   "non utf8 label value",
			labels: Labels{"a": "\xFF"},
		},
		{
			desc:   "not enough label values",
			labels: Labels{},
		},
		{
			desc:   "too many label values",
			labels: Labels{"a": "1", "b": "2"},
		},
	}

	for _, test := range testCases {
		metricDesc := NewDesc(
			"sample_value",
			"sample value",
			[]string{"a"},
			Labels{},
		)

		expectPanic(t, func() {
			MustNewConstMetric(metricDesc, CounterValue, 0.3, "\xFF")
		}, "WithLabelValues: expected panic because: "+test.desc)

		if _, err := NewConstMetric(metricDesc, CounterValue, 0.3, "\xFF"); err == nil {
			t.Errorf("NewConstMetric: expected error because: %s", test.desc)
		}
	}
}

func TestNewConstMetricWithCreatedTimestamp(t *testing.T) {
	now := time.Now()

	for _, tcase := range []struct {
		desc             string
		metricType       ValueType
		createdTimestamp time.Time
		expecErr         bool
		expectedCt       *timestamppb.Timestamp
	}{
		{
			desc:             "gauge with CT",
			metricType:       GaugeValue,
			createdTimestamp: now,
			expecErr:         true,
			expectedCt:       nil,
		},
		{
			desc:             "counter with CT",
			metricType:       CounterValue,
			createdTimestamp: now,
			expecErr:         false,
			expectedCt:       timestamppb.New(now),
		},
	} {
		t.Run(tcase.desc, func(t *testing.T) {
			metricDesc := NewDesc(
				"sample_value",
				"sample value",
				nil,
				nil,
			)
			m, err := NewConstMetricWithCreatedTimestamp(metricDesc, tcase.metricType, float64(1), tcase.createdTimestamp)
			if tcase.expecErr && err == nil {
				t.Errorf("Expected error is test %s, got no err", tcase.desc)
			}
			if !tcase.expecErr && err != nil {
				t.Errorf("Didn't expect error in test %s, got %s", tcase.desc, err.Error())
			}

			if tcase.expectedCt != nil {
				var metric dto.Metric
				m.Write(&metric)
				if metric.Counter.CreatedTimestamp.AsTime() != tcase.expectedCt.AsTime() {
					t.Errorf("Expected timestamp %v, got %v", tcase.expectedCt, &metric.Counter.CreatedTimestamp)
				}
			}
		})
	}
}

func TestMakeLabelPairs(t *testing.T) {
	tests := []struct {
		name        string
		desc        *Desc
		labelValues []string
		want        []*dto.LabelPair
	}{
		{
			name:        "no labels",
			desc:        NewDesc("metric-1", "", nil, nil),
			labelValues: nil,
			want:        nil,
		},
		{
			name: "only constant labels",
			desc: NewDesc("metric-1", "", nil, map[string]string{
				"label-1": "1",
				"label-2": "2",
				"label-3": "3",
			}),
			labelValues: nil,
			want: []*dto.LabelPair{
				{Name: proto.String("label-1"), Value: proto.String("1")},
				{Name: proto.String("label-2"), Value: proto.String("2")},
				{Name: proto.String("label-3"), Value: proto.String("3")},
			},
		},
		{
			name:        "only variable labels",
			desc:        NewDesc("metric-1", "", []string{"var-label-1", "var-label-2", "var-label-3"}, nil),
			labelValues: []string{"1", "2", "3"},
			want: []*dto.LabelPair{
				{Name: proto.String("var-label-1"), Value: proto.String("1")},
				{Name: proto.String("var-label-2"), Value: proto.String("2")},
				{Name: proto.String("var-label-3"), Value: proto.String("3")},
			},
		},
		{
			name: "variable and const labels",
			desc: NewDesc("metric-1", "", []string{"var-label-1", "var-label-2", "var-label-3"}, map[string]string{
				"label-1": "1",
				"label-2": "2",
				"label-3": "3",
			}),
			labelValues: []string{"1", "2", "3"},
			want: []*dto.LabelPair{
				{Name: proto.String("label-1"), Value: proto.String("1")},
				{Name: proto.String("label-2"), Value: proto.String("2")},
				{Name: proto.String("label-3"), Value: proto.String("3")},
				{Name: proto.String("var-label-1"), Value: proto.String("1")},
				{Name: proto.String("var-label-2"), Value: proto.String("2")},
				{Name: proto.String("var-label-3"), Value: proto.String("3")},
			},
		},
		{
			name: "unsorted variable and const labels are sorted",
			desc: NewDesc("metric-1", "", []string{"var-label-3", "var-label-2", "var-label-1"}, map[string]string{
				"label-3": "3",
				"label-2": "2",
				"label-1": "1",
			}),
			labelValues: []string{"3", "2", "1"},
			want: []*dto.LabelPair{
				{Name: proto.String("label-1"), Value: proto.String("1")},
				{Name: proto.String("label-2"), Value: proto.String("2")},
				{Name: proto.String("label-3"), Value: proto.String("3")},
				{Name: proto.String("var-label-1"), Value: proto.String("1")},
				{Name: proto.String("var-label-2"), Value: proto.String("2")},
				{Name: proto.String("var-label-3"), Value: proto.String("3")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MakeLabelPairs(tt.desc, tt.labelValues); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%v != %v", got, tt.want)
			}
		})
	}
}

var new1LabelDescFunc = func() *Desc {
	return NewDesc(
		"metric",
		"help",
		[]string{"var-label-1"},
		Labels{"const-label-1": "value"})
}

var new3LabelsDescFunc = func() *Desc {
	return NewDesc(
		"metric",
		"help",
		[]string{"var-label-1", "var-label-3", "var-label-2"},
		Labels{"const-label-1": "value", "const-label-3": "value", "const-label-2": "value"})
}

var new10LabelsDescFunc = func() *Desc {
	return NewDesc(
		"metric",
		"help",
		[]string{"var-label-5", "var-label-1", "var-label-3", "var-label-2", "var-label-10", "var-label-4", "var-label-7", "var-label-8", "var-label-9", "var-label-6"},
		Labels{"const-label-4": "value", "const-label-1": "value", "const-label-7": "value", "const-label-2": "value", "const-label-9": "value", "const-label-8": "value", "const-label-10": "value", "const-label-3": "value", "const-label-6": "value", "const-label-5": "value"})
}

func BenchmarkMakeLabelPairs(b *testing.B) {
	for _, bm := range []struct {
		desc                *Desc
		makeLabelPairValues []string
	}{
		{
			desc:                new1LabelDescFunc(),
			makeLabelPairValues: []string{"value"},
		},
		{
			desc:                new3LabelsDescFunc(),
			makeLabelPairValues: []string{"value", "value", "value"},
		},
		{
			desc:                new10LabelsDescFunc(),
			makeLabelPairValues: []string{"value", "value", "value", "value", "value", "value", "value", "value", "value", "value"},
		},
	} {
		b.Run(fmt.Sprintf("labels=%v", len(bm.makeLabelPairValues)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				MakeLabelPairs(bm.desc, bm.makeLabelPairValues)
			}
		})
	}
}

func BenchmarkConstMetricFlow(b *testing.B) {
	for _, bm := range []struct {
		descFunc    func() *Desc
		labelValues []string
	}{
		{
			descFunc:    new1LabelDescFunc,
			labelValues: []string{"value"},
		},
		{
			descFunc:    new3LabelsDescFunc,
			labelValues: []string{"value", "value", "value"},
		},
		{
			descFunc:    new10LabelsDescFunc,
			labelValues: []string{"value", "value", "value", "value", "value", "value", "value", "value", "value", "value"},
		},
	} {
		b.Run(fmt.Sprintf("labels=%v", len(bm.labelValues)), func(b *testing.B) {
			for _, metricsToCreate := range []int{1, 2, 3, 5} {
				b.Run(fmt.Sprintf("metrics=%v", metricsToCreate), func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						desc := bm.descFunc()
						for j := 0; j < metricsToCreate; j++ {
							_, err := NewConstMetric(desc, GaugeValue, 1.0, bm.labelValues...)
							if err != nil {
								b.Fatal(err)
							}
						}
					}
				})
			}
		})
	}
}
