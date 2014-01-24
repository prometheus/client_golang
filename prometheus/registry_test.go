// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package prometheus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"code.google.com/p/goprotobuf/proto"
)

func testRegister(t tester) {
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
					baseLabels: map[string]string{"name": "illegal_duplicate_name"},
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
				t.Errorf("%d.%d. expected %s, got %s", i, j, scenario.outputs[j], actual)
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

func testDecorateWriter(t tester) {
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

func testDumpToWriter(t tester) {
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
			out: []byte(`[{"name":"foo","baseLabels":{"label_foo":"foo"},"docstring":"metric foo","metric":{"type":"counter","value":[]}}]`),
		},
		{
			in: input{
				metrics: map[string]Metric{
					"foo": NewCounter(),
					"bar": NewCounter(),
				},
			},
			out: []byte(`[{"name":"bar","baseLabels":{"label_bar":"bar"},"docstring":"metric bar","metric":{"type":"counter","value":[]}},{"name":"foo","baseLabels":{"label_foo":"foo"},"docstring":"metric foo","metric":{"type":"counter","value":[]}}]`),
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
