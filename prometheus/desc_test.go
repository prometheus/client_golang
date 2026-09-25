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
	"errors"
	"reflect"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestNewDescInvalidConstLabelValues(t *testing.T) {
	labelValue := "\xFF"
	desc := NewDesc(
		"sample_label",
		"sample label",
		nil,
		Labels{"a": labelValue},
	)
	if desc.Err() == nil {
		t.Errorf("NewDesc: expected error because const label value is invalid: %s", labelValue)
	}
}

func TestNewDescInvalidVariableLabelName(t *testing.T) {
	labelValue := "__label__"
	desc := NewDesc(
		"sample_label",
		"sample label",
		[]string{labelValue},
		Labels{"a": "b"},
	)
	if desc.Err() == nil {
		t.Errorf("NewDesc: expected error because variable label name is invalid: %s", labelValue)
	}
}

func TestNewDescNilLabelValues(t *testing.T) {
	desc := NewDesc(
		"sample_label",
		"sample label",
		nil,
		nil,
	)
	if desc.Err() != nil {
		t.Errorf("NewDesc: unexpected error: %s", desc.Err())
	}
}

func TestNewDescWithNilLabelValues_String(t *testing.T) {
	desc := NewDesc(
		"sample_label",
		"sample label",
		nil,
		nil,
	)
	if desc.String() != `Desc{fqName: "sample_label", help: "sample label", unit: "", constLabels: {}, variableLabels: {}}` {
		t.Errorf("String: unexpected output: %s", desc.String())
	}
}

func TestNewInvalidDesc_String(t *testing.T) {
	desc := NewInvalidDesc(
		nil,
	)
	if desc.String() != `Desc{fqName: "", help: "", unit: "", constLabels: {}, variableLabels: {}}` {
		t.Errorf("String: unexpected output: %s", desc.String())
	}
}

func TestNewDescWithUnit_String(t *testing.T) {
	desc := V2.NewDesc(
		"sample_metric_bytes",
		"sample metric with unit",
		UnconstrainedLabels(nil),
		nil,
		WithUnit("bytes"),
	)
	if desc.String() != `Desc{fqName: "sample_metric_bytes", help: "sample metric with unit", unit: "bytes", constLabels: {}, variableLabels: {}}` {
		t.Errorf("String: unexpected output:\ngot:  %s\nwant: %s", desc.String(), desc.String())
	}
}

func TestDescInfo(t *testing.T) {
	for _, tc := range []struct {
		name string
		desc *Desc
		want DescInfo
	}{
		{
			name: "NewDesc carries no type",
			desc: NewDesc("no_type", "help", []string{"var"}, Labels{"const": "value"}),
			want: DescInfo{
				FQName:         "no_type",
				Help:           "help",
				Type:           dto.MetricType_UNTYPED,
				VariableLabels: []string{"var"},
				ConstLabels:    Labels{"const": "value"},
			},
		},
		{
			name: "counter",
			desc: NewCounter(CounterOpts{Name: "counted_total", Help: "help", Unit: "s"}).Desc(),
			want: DescInfo{
				FQName: "counted_total",
				Help:   "help",
				Unit:   "s",
				Type:   dto.MetricType_COUNTER,
			},
		},
		{
			name: "gauge vec",
			desc: NewGaugeVec(GaugeOpts{Name: "gauged", Help: "help"}, []string{"a", "b"}).
				WithLabelValues("1", "2").Desc(),
			want: DescInfo{
				FQName:         "gauged",
				Help:           "help",
				Type:           dto.MetricType_GAUGE,
				VariableLabels: []string{"a", "b"},
			},
		},
		{
			name: "histogram",
			desc: NewHistogram(HistogramOpts{Name: "observed", Help: "help"}).Desc(),
			want: DescInfo{FQName: "observed", Help: "help", Type: dto.MetricType_HISTOGRAM},
		},
		{
			name: "summary",
			desc: NewSummary(SummaryOpts{Name: "summarized", Help: "help"}).Desc(),
			want: DescInfo{FQName: "summarized", Help: "help", Type: dto.MetricType_SUMMARY},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.desc.Info()
			if got.Err != nil {
				t.Fatalf("unexpected error: %s", got.Err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDescInfoOfInvalidDesc(t *testing.T) {
	// NewInvalidDesc leaves variableLabels nil, so Info must not panic on it.
	info := NewInvalidDesc(errors.New("boom")).Info()
	if info.Err == nil {
		t.Fatal("expected Info to report the construction error")
	}
	if info.Type != dto.MetricType_UNTYPED {
		t.Errorf("got type %s, want UNTYPED", info.Type)
	}
}

func TestDescInfoDoesNotAliasDesc(t *testing.T) {
	desc := NewDesc("aliased", "help", []string{"var"}, Labels{"const": "value"})

	info := desc.Info()
	info.VariableLabels[0] = "mutated"
	info.ConstLabels["const"] = "mutated"

	fresh := desc.Info()
	if fresh.VariableLabels[0] != "var" {
		t.Errorf("mutating the returned VariableLabels changed the Desc: %v", fresh.VariableLabels)
	}
	if fresh.ConstLabels["const"] != "value" {
		t.Errorf("mutating the returned ConstLabels changed the Desc: %v", fresh.ConstLabels)
	}
}

func TestWrappedDescKeepsType(t *testing.T) {
	reg := NewPedanticRegistry()
	WrapRegistererWithPrefix("wrapped_", reg).
		MustRegister(NewCounter(CounterOpts{Name: "counted_total", Help: "help"}))

	descs := reg.DescribeAll()
	if len(descs) != 1 {
		t.Fatalf("got %d descs, want 1", len(descs))
	}
	info := descs[0].Info()
	if info.FQName != "wrapped_counted_total" {
		t.Errorf("got name %q, want %q", info.FQName, "wrapped_counted_total")
	}
	if info.Type != dto.MetricType_COUNTER {
		t.Errorf("got type %s, want COUNTER", info.Type)
	}
}
