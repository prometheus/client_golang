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

func testCustom(t tester) {
	type input struct {
		steps []func(g Custom)
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
				steps: []func(g Custom){},
			},
			out: output{
				value: `{"type":"custom","value":[]}`,
			},
		},
		{
			in: input{
				steps: []func(g Custom){
					func(g Custom) {
						g.Set(nil, 1)
					},
				},
			},
			out: output{
				value: `{"type":"custom","value":[{"labels":{},"value":1}]}`,
			},
		},
		{
			in: input{
				steps: []func(g Custom){
					func(g Custom) {
						g.Set(map[string]string{}, 2)
					},
				},
			},
			out: output{
				value: `{"type":"custom","value":[{"labels":{},"value":2}]}`,
			},
		},
		{
			in: input{
				steps: []func(g Custom){
					func(g Custom) {
						g.Set(map[string]string{}, 3)
					},
					func(g Custom) {
						g.Set(map[string]string{}, 5)
					},
				},
			},
			out: output{
				value: `{"type":"custom","value":[{"labels":{},"value":5}]}`,
			},
		},
		{
			in: input{
				steps: []func(g Custom){
					func(g Custom) {
						g.Set(map[string]string{"handler": "/foo"}, 13)
					},
					func(g Custom) {
						g.Set(map[string]string{"handler": "/bar"}, 17)
					},
					func(g Custom) {
						g.Reset(map[string]string{"handler": "/bar"})
					},
				},
			},
			out: output{
				value: `{"type":"custom","value":[{"labels":{"handler":"/foo"},"value":13}]}`,
			},
		},
		{
			in: input{
				steps: []func(g Custom){
					func(g Custom) {
						g.Set(map[string]string{"handler": "/foo"}, 13)
					},
					func(g Custom) {
						g.Set(map[string]string{"handler": "/bar"}, 17)
					},
					func(g Custom) {
						g.ResetAll()
					},
				},
			},
			out: output{
				value: `{"type":"custom","value":[]}`,
			},
		},
		{
			in: input{
				steps: []func(g Custom){
					func(g Custom) {
						g.Set(map[string]string{"handler": "/foo"}, 19)
					},
				},
			},
			out: output{
				value: `{"type":"custom","value":[{"labels":{"handler":"/foo"},"value":19}]}`,
			},
		},
	}

	for i, scenario := range scenarios {
		custom := NewCustom()

		for _, step := range scenario.in.steps {
			step(custom)
		}

		bytes, err := json.Marshal(custom)
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

func TestCustom(t *testing.T) {
	testCustom(t)
}

func BenchmarkCustom(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testCustom(b)
	}
}
