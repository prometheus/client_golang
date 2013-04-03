// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"testing"
)

func testLabelsToSignature(t tester) {
	var scenarios = []struct {
		in  map[string]string
		out string
	}{
		{
			in:  map[string]string{},
			out: "",
		},
		{},
	}

	for i, scenario := range scenarios {
		actual := labelsToSignature(scenario.in)

		if actual != scenario.out {
			t.Errorf("%d. expected %s, got %s", i, scenario.out, actual)
		}
	}
}

func TestLabelToSignature(t *testing.T) {
	testLabelsToSignature(t)
}

func BenchmarkLabelToSignature(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testLabelsToSignature(b)
	}
}
