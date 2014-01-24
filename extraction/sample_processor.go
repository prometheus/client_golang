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
	"io"

	"github.com/matttproud/golang_protobuf_extensions/ext"
	"github.com/prometheus/client_golang/model"
	dto "github.com/prometheus/client_model/go"
)

type sampleProcessor struct{}

// SampleProcessor decodes varint encoded record length-delimited streams
// of io.prometheus.client.Sample.
//
// See http://godoc.org/github.com/matttproud/golang_protobuf_extensions/ext for
// more details.
var SampleProcessor = new(sampleProcessor)

type sampleBuffer struct {
	buffer model.Samples
	out    Ingester
}

// newSampleBuffer returns a new sample buffer of the given size which Flushes
// samples to out as the buffer fills.
func newSampleBuffer(size int, out Ingester) *sampleBuffer {
	return &sampleBuffer{
		buffer: make(model.Samples, 0, size),
		out:    out,
	}
}

// Append adds a sample to the buffer. If the buffer is full, Flush will be
// called before the sample is added.
func (b *sampleBuffer) Append(sample *model.Sample) error {
	if len(b.buffer) == cap(b.buffer) {
		if err := b.Flush(); err != nil {
			return err
		}
	}

	b.buffer = append(b.buffer, sample)
	return nil
}

// Flush calls Ingest on the provided Ingester with the buffered samples and
// resets the buffer.
// Since the Ingester may or may not act immediately on the samples provided,
// we make a copy first.
func (b *sampleBuffer) Flush() error {
	if len(b.buffer) == 0 {
		return nil
	}

	samples := make(model.Samples, len(b.buffer))
	copy(samples, b.buffer)
	b.buffer = b.buffer[:0]

	return b.out.Ingest(&Result{Samples: samples})
}

func (sampleProcessor) ProcessSingle(i io.Reader, out Ingester, o *ProcessOptions) error {
	var (
		err error

		sample  = new(dto.Sample)
		samples = newSampleBuffer(64, out)
	)

	for {
		sample.Reset()

		if _, err = ext.ReadDelimited(i, sample); err != nil {
			if err == io.EOF {
				err = nil
				break
			}

			break
		}

		labels := make(model.Metric, len(sample.GetLabel())+1)

		for _, label := range sample.GetLabel() {
			labels[model.LabelName(label.GetKey())] = model.LabelValue(label.GetVal())
		}

		labels[model.MetricNameLabel] = model.LabelValue(sample.GetName())

		err = samples.Append(&model.Sample{
			Metric:    model.Metric(labels),
			Timestamp: o.Timestamp,
			Value:     model.SampleValue(sample.GetValue()),
		})

		if err != nil {
			return err
		}
	}

	if err != nil {
		return err
	}

	return samples.Flush()
}
