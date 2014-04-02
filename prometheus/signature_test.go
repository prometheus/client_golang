// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"runtime"
	"testing"
)

func testLabelsToSignature(t tester) {
	var scenarios = []struct {
		in  map[string]string
		out uint64
	}{
		{
			in:  map[string]string{},
			out: 14695981039346656037,
		},
		{
			in:  map[string]string{"name": "garland, briggs", "fear": "love is not enough"},
			out: 15753083015552662396,
		},
	}

	for i, scenario := range scenarios {
		actual := labelsToSignature(scenario.in)

		if actual != scenario.out {
			t.Errorf("%d. expected %d, got %d", i, scenario.out, actual)
		}
	}
}

func TestLabelToSignature(t *testing.T) {
	testLabelsToSignature(t)
}

func TestEmptyLabelSignature(t *testing.T) {
	input := []map[string]string{nil, {}}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	alloc := ms.Alloc

	for _, labels := range input {
		labelsToSignature(labels)
	}

	runtime.ReadMemStats(&ms)

	if got := ms.Alloc; alloc != got {
		t.Fatal("expected labelsToSignature with empty labels not to perform allocations")
	}
}

func BenchmarkLabelToSignature(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testLabelsToSignature(b)
	}
}
