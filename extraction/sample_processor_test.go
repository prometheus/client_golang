// Copyright 2013 Prometheus Team
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

package extraction

import (
	"bytes"
	"testing"

	"code.google.com/p/goprotobuf/proto"
	"github.com/matttproud/golang_protobuf_extensions/ext"
	"github.com/prometheus/client_golang/model"
	dto "github.com/prometheus/client_model/go"
)

type results []*Result

func (rs *results) Ingest(r *Result) error {
	*rs = append(*rs, r)

	return nil
}

func TestSampleProcessor(t *testing.T) {
	var (
		buf     = new(bytes.Buffer)
		results = results{}
		options = &ProcessOptions{Timestamp: model.Now()}
	)

	ext.WriteDelimited(buf, &dto.Sample{
		Name:  proto.String("request_count"),
		Value: proto.Float64(-42),
		Label: []*dto.Label{
			{Key: proto.String("label_name"), Val: proto.String("label_value")},
		},
	})

	ext.WriteDelimited(buf, &dto.Sample{
		Name:  proto.String("request_count"),
		Value: proto.Float64(6.4),
		Label: []*dto.Label{
			{Key: proto.String("another_label_name"), Val: proto.String("another_label_value")},
		},
	})

	if err := SampleProcessor.ProcessSingle(buf, &results, options); err != nil {
		t.Fatal(err)
	}

	if expected, got := 1, len(results); expected != got {
		t.Fatal("expected %d results, got %d", expected, got)
	}

	expected := &Result{
		Samples: model.Samples{
			{
				Metric:    model.Metric{"name": "request_count", "label_name": "label_value"},
				Timestamp: options.Timestamp,
				Value:     -42,
			},
			{
				Metric:    model.Metric{"name": "request_count", "another_label_name": "another_label_value"},
				Timestamp: options.Timestamp,
				Value:     6.4,
			},
		},
	}

	if !expected.equal(results[0]) {
		t.Fatalf("expected %#v, got %#v", expected, results[0])
	}
}

func TestSampleBuffer(t *testing.T) {
	var (
		results = results{}
		samples = newSampleBuffer(5, &results)
	)

	for i := 0; i < 5; i++ {
		if err := samples.Append(&model.Sample{Value: 1}); err != nil {
			t.Fatal("unexpected error:", err)
		}
	}

	if len(results) != 0 {
		t.Fatal("sample buffer unexpectedly flushed")
	}

	if err := samples.Append(&model.Sample{Value: -1}); err != nil {
		t.Fatal("unexpected error:", err)
	}

	if len(results) != 1 {
		t.Fatal("sample buffer not flushed")
	}

	if len(results[0].Samples) != 5 {
		t.Fatalf("expected 5 flushed samples, got %d", len(results[0].Samples))
	}

	for _, s := range results[0].Samples {
		if s.Value != 1 {
			t.Fatal("incorrectly flushed samples")
		}
	}

	samples.Flush()

	if len(results) != 2 {
		t.Fatal("sample buffer not flushed")
	}

	if len(results[1].Samples) != 1 {
		t.Fatalf("expected 1 flushed samples, got %d", len(results[1].Samples))
	}

	for _, s := range results[1].Samples {
		if s.Value != -1 {
			t.Fatal("incorrectly flushed samples")
		}
	}

	samples.Flush()

	if len(results) != 2 {
		t.Fatal("sample buffer flushed with no values")
	}
}
