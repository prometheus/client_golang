// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package prometheus

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	dto "github.com/prometheus/client_model/go"

	"code.google.com/p/goprotobuf/proto"

	"github.com/prometheus/client_golang/model"
	"github.com/prometheus/client_golang/test"
)

func testRegister(t test.Tester) {
	var oldState = struct {
		abortOnMisuse             bool
		debugRegistration         bool
		useAggressiveSanityChecks bool
	}{
		abortOnMisuse:             *abortOnMisuse,
		debugRegistration:         *debugRegistration,
		useAggressiveSanityChecks: *useAggressiveSanityChecks,
	}
	defer func() {
		abortOnMisuse = &(oldState.abortOnMisuse)
		debugRegistration = &(oldState.debugRegistration)
		useAggressiveSanityChecks = &(oldState.useAggressiveSanityChecks)
	}()

	type input struct {
		name       string
		baseLabels map[string]string
	}

	validLabels := map[string]string{"label": "value"}

	var scenarios = []struct {
		inputs  []input
		outputs []bool
	}{
		{},
		{
			inputs: []input{
				{
					name: "my_name_without_labels",
				},
			},
			outputs: []bool{
				true,
			},
		},
		{
			inputs: []input{
				{
					name: "my_name_without_labels",
				},
				{
					name: "another_name_without_labels",
				},
			},
			outputs: []bool{
				true,
				true,
			},
		},
		{
			inputs: []input{
				{
					name: "",
				},
			},
			outputs: []bool{
				false,
			},
		},
		{
			inputs: []input{
				{
					name:       "valid_name",
					baseLabels: map[string]string{model.ReservedLabelPrefix + "internal": "illegal_internal_name"},
				},
			},
			outputs: []bool{
				false,
			},
		},
		{
			inputs: []input{
				{
					name: "duplicate_names",
				},
				{
					name: "duplicate_names",
				},
			},
			outputs: []bool{
				true,
				false,
			},
		},
		{
			inputs: []input{
				{
					name:       "duplicate_names_with_identical_labels",
					baseLabels: map[string]string{"label": "value"},
				},
				{
					name:       "duplicate_names_with_identical_labels",
					baseLabels: map[string]string{"label": "value"},
				},
			},
			outputs: []bool{
				true,
				false,
			},
		},
		{
			inputs: []input{
				{
					name:       "metric_1_with_identical_labels",
					baseLabels: validLabels,
				},
				{
					name:       "metric_2_with_identical_labels",
					baseLabels: validLabels,
				},
			},
			outputs: []bool{
				true,
				true,
			},
		},
		{
			inputs: []input{
				{
					name:       "duplicate_names_with_dissimilar_labels",
					baseLabels: map[string]string{"label": "foo"},
				},
				{
					name:       "duplicate_names_with_dissimilar_labels",
					baseLabels: map[string]string{"label": "bar"},
				},
			},
			outputs: []bool{
				true,
				false,
			},
		},
	}

	for i, scenario := range scenarios {
		if len(scenario.inputs) != len(scenario.outputs) {
			t.Fatalf("%d. expected scenario output length %d, got %d", i, len(scenario.inputs), len(scenario.outputs))
		}

		abortOnMisuse = proto.Bool(false)
		debugRegistration = proto.Bool(false)
		useAggressiveSanityChecks = proto.Bool(true)

		registry := NewRegistry()

		for j, input := range scenario.inputs {
			actual := registry.Register(input.name, "", input.baseLabels, nil)
			if scenario.outputs[j] != (actual == nil) {
				t.Errorf("%d.%d. expected %t, got %t", i, j, scenario.outputs[j], actual == nil)
			}
		}
	}
}

func TestRegister(t *testing.T) {
	testRegister(t)
}

func BenchmarkRegister(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testRegister(b)
	}
}

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

func testHandler(t test.Tester) {

	metric := NewCounter()
	metric.Increment(map[string]string{"labelname": "val1"})
	metric.Increment(map[string]string{"labelname": "val2"})

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
		registry := NewRegistry().(*registry)
		if scenario.withCounter {
			registry.Register(
				"name", "docstring",
				map[string]string{"basename": "basevalue"},
				metric,
			)
		}
		if scenario.withExternalMF {
			registry.SetMetricFamilyInjectionHook(
				func() []*dto.MetricFamily {
					return externalMetricFamily
				},
			)
		}
		writer := &fakeResponseWriter{
			header: http.Header{},
		}
		handler := registry.Handler()
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

func TestHander(t *testing.T) {
	testHandler(t)
}

func BenchmarkHandler(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testHandler(b)
	}
}

func testDecorateWriter(t test.Tester) {
	type input struct {
		headers map[string]string
		body    []byte
	}

	type output struct {
		headers map[string]string
		body    []byte
	}

	var scenarios = []struct {
		in  input
		out output
	}{
		{},
		{
			in: input{
				headers: map[string]string{
					"Accept-Encoding": "gzip,deflate,sdch",
				},
				body: []byte("Hi, mom!"),
			},
			out: output{
				headers: map[string]string{
					"Content-Encoding": "gzip",
				},
				body: []byte("\x1f\x8b\b\x00\x00\tn\x88\x00\xff\xf2\xc8\xd4Q\xc8\xcd\xcfU\x04\x04\x00\x00\xff\xff9C&&\b\x00\x00\x00"),
			},
		},
		{
			in: input{
				headers: map[string]string{
					"Accept-Encoding": "foo",
				},
				body: []byte("Hi, mom!"),
			},
			out: output{
				headers: map[string]string{},
				body:    []byte("Hi, mom!"),
			},
		},
	}

	for i, scenario := range scenarios {
		request, _ := http.NewRequest("GET", "/", nil)

		for key, value := range scenario.in.headers {
			request.Header.Add(key, value)
		}

		baseWriter := &fakeResponseWriter{
			header: make(http.Header),
		}

		writer := decorateWriter(request, baseWriter)

		for key, value := range scenario.out.headers {
			if baseWriter.Header().Get(key) != value {
				t.Errorf("%d. expected %s for header %s, got %s", i, value, key, baseWriter.Header().Get(key))
			}
		}

		writer.Write(scenario.in.body)

		if closer, ok := writer.(io.Closer); ok {
			closer.Close()
		}

		if !bytes.Equal(scenario.out.body, baseWriter.body.Bytes()) {
			t.Errorf("%d. expected %s for body, got %s", i, scenario.out.body, baseWriter.body.Bytes())
		}
	}
}

func TestDecorateWriter(t *testing.T) {
	testDecorateWriter(t)
}

func BenchmarkDecorateWriter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testDecorateWriter(b)
	}
}

func testDumpToWriter(t test.Tester) {
	type input struct {
		metrics map[string]Metric
	}

	var scenarios = []struct {
		in  input
		out []byte
	}{
		{
			out: []byte("[]"),
		},
		{
			in: input{
				metrics: map[string]Metric{
					"foo": NewCounter(),
				},
			},
			out: []byte(`[{"baseLabels":{"__name__":"foo","label_foo":"foo"},"docstring":"metric foo","metric":{"type":"counter","value":[]}}]`),
		},
		{
			in: input{
				metrics: map[string]Metric{
					"foo": NewCounter(),
					"bar": NewCounter(),
				},
			},
			out: []byte(`[{"baseLabels":{"__name__":"bar","label_bar":"bar"},"docstring":"metric bar","metric":{"type":"counter","value":[]}},{"baseLabels":{"__name__":"foo","label_foo":"foo"},"docstring":"metric foo","metric":{"type":"counter","value":[]}}]`),
		},
	}

	for i, scenario := range scenarios {
		registry := NewRegistry().(*registry)

		for name, metric := range scenario.in.metrics {
			err := registry.Register(name, fmt.Sprintf("metric %s", name), map[string]string{fmt.Sprintf("label_%s", name): name}, metric)
			if err != nil {
				t.Errorf("%d. encountered error while registering metric %s", i, err)
			}
		}

		actual, err := json.Marshal(registry)

		if err != nil {
			t.Errorf("%d. encountered error while dumping %s", i, err)
		}

		if !bytes.Equal(scenario.out, actual) {
			t.Errorf("%d. expected %q for dumping, got %q", i, scenario.out, actual)
		}
	}
}

func TestDumpToWriter(t *testing.T) {
	testDumpToWriter(t)
}

func BenchmarkDumpToWriter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testDumpToWriter(b)
	}
}

func ExampleMustRegister() {
	var gauge = NewGauge(GaugeDesc{
		Desc{
			Name: "my_spiffy_metric",
			Help: "it's spiffy description",
		},
	})

	MustRegister(gauge)
}

func ExampleMustRegisterOrGet() {
	// I may have already registered this.
	var gauge = MustRegisterOrGet(NewGauge(GaugeDesc{
		Desc{
			Name: "my_spiffy_metric",
			Help: "it's spiffy description",
		},
	})).(Gauge)

	gauge.Set(42)
}

func ExampleUnregister() {
	var oldAndBusted Gauge // I no longer need this!
	Unregister(oldAndBusted)
}

func ExampleHandler() {
	http.Handle("/metrics", Handler) // Easy!
}
