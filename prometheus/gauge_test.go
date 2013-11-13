// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"encoding/json"
	"testing"
)

func testGauge(t tester) {
	type input struct {
		steps []func(g Gauge)
	}
	type output struct {
		value string
	}

	var scenarios = []struct {
		in  input
		out output
	}{
		{
			in: input{
				steps: []func(g Gauge){},
			},
			out: output{
				value: `{"type":"gauge","value":[]}`,
			},
		},
		{
			in: input{
				steps: []func(g Gauge){
					func(g Gauge) {
						g.Set(nil, 1)
					},
				},
			},
			out: output{
				value: `{"type":"gauge","value":[{"labels":{},"value":1}]}`,
			},
		},
		{
			in: input{
				steps: []func(g Gauge){
					func(g Gauge) {
						g.Set(map[string]string{}, 2)
					},
				},
			},
			out: output{
				value: `{"type":"gauge","value":[{"labels":{},"value":2}]}`,
			},
		},
		{
			in: input{
				steps: []func(g Gauge){
					func(g Gauge) {
						g.Set(map[string]string{}, 3)
					},
					func(g Gauge) {
						g.Set(map[string]string{}, 5)
					},
				},
			},
			out: output{
				value: `{"type":"gauge","value":[{"labels":{},"value":5}]}`,
			},
		},
		{
			in: input{
				steps: []func(g Gauge){
					func(g Gauge) {
						g.Set(map[string]string{"path": "/foo"}, 13)
					},
					func(g Gauge) {
						g.Set(map[string]string{"path": "/bar"}, 17)
					},
					func(g Gauge) {
						g.ResetAll()
					},
				},
			},
			out: output{
				value: `{"type":"gauge","value":[]}`,
			},
		},
		{
			in: input{
				steps: []func(g Gauge){
					func(g Gauge) {
						g.Set(map[string]string{"path": "/foo"}, 19)
					},
				},
			},
			out: output{
				value: `{"type":"gauge","value":[{"labels":{"path":"/foo"},"value":19}]}`,
			},
		},
	}

	for i, scenario := range scenarios {
		gauge := NewGauge()

		for _, step := range scenario.in.steps {
			step(gauge)
		}

		bytes, err := json.Marshal(gauge)
		if err != nil {
			t.Errorf("%d. could not marshal into JSON %s", i, err)
			continue
		}

		asString := string(bytes)

		if scenario.out.value != asString {
			t.Errorf("%d. expected %q, got %q", i, scenario.out.value, asString)
		}
	}
}

func TestGauge(t *testing.T) {
	testGauge(t)
}

func BenchmarkGauge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testGauge(b)
	}
}
