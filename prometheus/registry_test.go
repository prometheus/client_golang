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

// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package prometheus

import (
	"bytes"
	"encoding/binary"
	"net/http"
	"testing"

	"code.google.com/p/goprotobuf/proto"
	dto "github.com/prometheus/client_model/go"
)

type fakeResponseWriter struct {
	header http.Header
	body   bytes.Buffer
}

func (r *fakeResponseWriter) Header() http.Header {
	return r.header
}

func (r *fakeResponseWriter) Write(d []byte) (l int, err error) {
	return r.body.Write(d)
}

func (r *fakeResponseWriter) WriteHeader(c int) {
}

func testHandler(t testing.TB) {

	metricVec := MustNewCounterVec(&Desc{
		Name:           "name",
		Help:           "docstring",
		VariableLabels: []string{"labelname"},
	})

	metricVec.WithLabelValues("val1").Inc()
	metricVec.WithLabelValues("val2").Inc()

	varintBuf := make([]byte, binary.MaxVarintLen32)

	externalMetricFamily := []*dto.MetricFamily{
		{
			Name: proto.String("externalname"),
			Help: proto.String("externaldocstring"),
			Type: dto.MetricType_COUNTER.Enum(),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{
							Name:  proto.String("externallabelname"),
							Value: proto.String("externalval1"),
						},
						{
							Name:  proto.String("externalbasename"),
							Value: proto.String("externalbasevalue"),
						},
					},
					Counter: &dto.Counter{
						Value: proto.Float64(1),
					},
				},
			},
		},
	}
	marshaledExternalMetricFamily, err := proto.Marshal(externalMetricFamily[0])
	if err != nil {
		t.Fatal(err)
	}
	var externalBuf bytes.Buffer
	l := binary.PutUvarint(varintBuf, uint64(len(marshaledExternalMetricFamily)))
	_, err = externalBuf.Write(varintBuf[:l])
	if err != nil {
		t.Fatal(err)
	}
	_, err = externalBuf.Write(marshaledExternalMetricFamily)
	if err != nil {
		t.Fatal(err)
	}
	externalMetricFamilyAsBytes := externalBuf.Bytes()
	externalMetricFamilyAsText := []byte(`# HELP externalname externaldocstring
# TYPE externalname counter
externalname{externallabelname="externalval1",externalbasename="externalbasevalue"} 1
`)
	externalMetricFamilyAsProtoText := []byte(`name: "externalname"
help: "externaldocstring"
type: COUNTER
metric: <
  label: <
    name: "externallabelname"
    value: "externalval1"
  >
  label: <
    name: "externalbasename"
    value: "externalbasevalue"
  >
  counter: <
    value: 1
  >
>

`)
	externalMetricFamilyAsProtoCompactText := []byte(`name:"externalname" help:"externaldocstring" type:COUNTER metric:<label:<name:"externallabelname" value:"externalval1" > label:<name:"externalbasename" value:"externalbasevalue" > counter:<value:1 > > 
`)

	expectedMetricFamily := &dto.MetricFamily{
		Name: proto.String("name"),
		Help: proto.String("docstring"),
		Type: dto.MetricType_COUNTER.Enum(),
		Metric: []*dto.Metric{
			{
				Label: []*dto.LabelPair{
					{
						Name:  proto.String("labelname"),
						Value: proto.String("val1"),
					},
					{
						Name:  proto.String("basename"),
						Value: proto.String("basevalue"),
					},
				},
				Counter: &dto.Counter{
					Value: proto.Float64(1),
				},
			},
			{
				Label: []*dto.LabelPair{
					{
						Name:  proto.String("labelname"),
						Value: proto.String("val2"),
					},
					{
						Name:  proto.String("basename"),
						Value: proto.String("basevalue"),
					},
				},
				Counter: &dto.Counter{
					Value: proto.Float64(1),
				},
			},
		},
	}
	marshaledExpectedMetricFamily, err := proto.Marshal(expectedMetricFamily)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	l = binary.PutUvarint(varintBuf, uint64(len(marshaledExpectedMetricFamily)))
	_, err = buf.Write(varintBuf[:l])
	if err != nil {
		t.Fatal(err)
	}
	_, err = buf.Write(marshaledExpectedMetricFamily)
	if err != nil {
		t.Fatal(err)
	}
	expectedMetricFamilyAsBytes := buf.Bytes()
	expectedMetricFamilyAsText := []byte(`# HELP name docstring
# TYPE name counter
name{labelname="val1",basename="basevalue"} 1
name{labelname="val2",basename="basevalue"} 1
`)
	expectedMetricFamilyAsProtoText := []byte(`name: "name"
help: "docstring"
type: COUNTER
metric: <
  label: <
    name: "labelname"
    value: "val1"
  >
  label: <
    name: "basename"
    value: "basevalue"
  >
  counter: <
    value: 1
  >
>
metric: <
  label: <
    name: "labelname"
    value: "val2"
  >
  label: <
    name: "basename"
    value: "basevalue"
  >
  counter: <
    value: 1
  >
>

`)
	expectedMetricFamilyAsProtoCompactText := []byte(`name:"name" help:"docstring" type:COUNTER metric:<label:<name:"labelname" value:"val1" > label:<name:"basename" value:"basevalue" > counter:<value:1 > > metric:<label:<name:"labelname" value:"val2" > label:<name:"basename" value:"basevalue" > counter:<value:1 > > 
`)

	type output struct {
		headers map[string]string
		body    []byte
	}

	var scenarios = []struct {
		headers        map[string]string
		out            output
		withCounter    bool
		withExternalMF bool
	}{
		{ // 0
			headers: map[string]string{
				"Accept": "foo/bar;q=0.2, dings/bums;q=0.8",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/json; schema="prometheus/telemetry"; version=0.0.2`,
				},
				body: []byte("[]\n"),
			},
		},
		{ // 1
			headers: map[string]string{
				"Accept": "foo/bar;q=0.2, application/quark;q=0.8",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/json; schema="prometheus/telemetry"; version=0.0.2`,
				},
				body: []byte("[]\n"),
			},
		},
		{ // 2
			headers: map[string]string{
				"Accept": "foo/bar;q=0.2, application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=bla;q=0.8",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/json; schema="prometheus/telemetry"; version=0.0.2`,
				},
				body: []byte("[]\n"),
			},
		},
		{ // 3
			headers: map[string]string{
				"Accept": "text/plain;q=0.2, application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited;q=0.8",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="delimited"`,
				},
				body: []byte{},
			},
		},
		{ // 4
			headers: map[string]string{
				"Accept": "application/json",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/json; schema="prometheus/telemetry"; version=0.0.2`,
				},
				body: []byte(`[{"baseLabels":{"__name__":"name","basename":"basevalue"},"docstring":"docstring","metric":{"type":"counter","value":[{"labels":{"labelname":"val1"},"value":1},{"labels":{"labelname":"val2"},"value":1}]}}]
`),
			},
			withCounter: true,
		},
		{ // 5
			headers: map[string]string{
				"Accept": "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="delimited"`,
				},
				body: expectedMetricFamilyAsBytes,
			},
			withCounter: true,
		},
		{ // 6
			headers: map[string]string{
				"Accept": "application/json",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/json; schema="prometheus/telemetry"; version=0.0.2`,
				},
				body: []byte("[]\n"),
			},
			withExternalMF: true,
		},
		{ // 7
			headers: map[string]string{
				"Accept": "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="delimited"`,
				},
				body: externalMetricFamilyAsBytes,
			},
			withExternalMF: true,
		},
		{ // 8
			headers: map[string]string{
				"Accept": "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="delimited"`,
				},
				body: bytes.Join(
					[][]byte{
						expectedMetricFamilyAsBytes,
						externalMetricFamilyAsBytes,
					},
					[]byte{},
				),
			},
			withCounter:    true,
			withExternalMF: true,
		},
		{ // 9
			headers: map[string]string{
				"Accept": "text/plain",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `text/plain; version=0.0.4`,
				},
				body: []byte{},
			},
		},
		{ // 10
			headers: map[string]string{
				"Accept": "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=bla;q=0.2, text/plain;q=0.5",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `text/plain; version=0.0.4`,
				},
				body: expectedMetricFamilyAsText,
			},
			withCounter: true,
		},
		{ // 11
			headers: map[string]string{
				"Accept": "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=bla;q=0.2, text/plain;q=0.5;version=0.0.4",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `text/plain; version=0.0.4`,
				},
				body: bytes.Join(
					[][]byte{
						expectedMetricFamilyAsText,
						externalMetricFamilyAsText,
					},
					[]byte{},
				),
			},
			withCounter:    true,
			withExternalMF: true,
		},
		{ // 12
			headers: map[string]string{
				"Accept": "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited;q=0.2, text/plain;q=0.5;version=0.0.2",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="delimited"`,
				},
				body: bytes.Join(
					[][]byte{
						expectedMetricFamilyAsBytes,
						externalMetricFamilyAsBytes,
					},
					[]byte{},
				),
			},
			withCounter:    true,
			withExternalMF: true,
		},
		{ // 13
			headers: map[string]string{
				"Accept": "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=text;q=0.5, application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited;q=0.4",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="text"`,
				},
				body: bytes.Join(
					[][]byte{
						expectedMetricFamilyAsProtoText,
						externalMetricFamilyAsProtoText,
					},
					[]byte{},
				),
			},
			withCounter:    true,
			withExternalMF: true,
		},
		{ // 14
			headers: map[string]string{
				"Accept": "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=compact-text",
			},
			out: output{
				headers: map[string]string{
					"Content-Type": `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="compact-text"`,
				},
				body: bytes.Join(
					[][]byte{
						expectedMetricFamilyAsProtoCompactText,
						externalMetricFamilyAsProtoCompactText,
					},
					[]byte{},
				),
			},
			withCounter:    true,
			withExternalMF: true,
		},
	}
	for i, scenario := range scenarios {
		registry := newRegistry()
		if scenario.withCounter {
			registry.Register(metricVec)
		}
		if scenario.withExternalMF {
			registry.metricFamilyInjectionHook = func() []*dto.MetricFamily {
				return externalMetricFamily
			}
		}
		writer := &fakeResponseWriter{
			header: http.Header{},
		}
		handler := InstrumentHandler("prometheus", registry)
		request, _ := http.NewRequest("GET", "/", nil)
		for key, value := range scenario.headers {
			request.Header.Add(key, value)
		}
		handler(writer, request)

		for key, value := range scenario.out.headers {
			if writer.Header().Get(key) != value {
				t.Errorf(
					"%d. expected %q for header %q, got %q",
					i, value, key, writer.Header().Get(key),
				)
			}
		}

		if !bytes.Equal(scenario.out.body, writer.body.Bytes()) {
			t.Errorf(
				"%d. expected %q for body, got %q",
				i, scenario.out.body, writer.body.Bytes(),
			)
		}
	}
}

// func TestHandler(t *testing.T) {
// 	testHandler(t)
// }
//
// func BenchmarkHandler(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		testHandler(b)
// 	}
// }

func ExampleMustRegister() {
	MustRegister(MustNewGauge(&Desc{
		Name: "my_spiffy_metric",
		Help: "it's spiffy description",
	}))
}

func ExampleMustRegisterOrGet() {
	// I may have already registered this.
	gauge := MustNewGauge(&Desc{
		Name: "my_spiffy_metric",
		Help: "it's spiffy description",
	})
	gauge = MustRegisterOrGet(gauge).(Gauge)
	gauge.Set(42)
}

func ExampleUnregister() {
	var oldAndBusted Gauge // I no longer need this!
	Unregister(oldAndBusted)
}

func ExampleHandler() {
	http.Handle("/metrics", Handler) // Easy!
}
