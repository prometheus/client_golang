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

package model

import (
	"runtime"
	"testing"
)

func TestFingerprintComparison(t *testing.T) {
	fingerprints := []*Fingerprint{
		{
			Hash: 0,
			FirstCharacterOfFirstLabelName: "b",
			LabelMatterLength:              1,
			LastCharacterOfLastLabelValue:  "b",
		},
		{
			Hash: 1,
			FirstCharacterOfFirstLabelName: "a",
			LabelMatterLength:              0,
			LastCharacterOfLastLabelValue:  "a",
		},
		{
			Hash: 1,
			FirstCharacterOfFirstLabelName: "a",
			LabelMatterLength:              1000,
			LastCharacterOfLastLabelValue:  "b",
		},
		{
			Hash: 1,
			FirstCharacterOfFirstLabelName: "b",
			LabelMatterLength:              0,
			LastCharacterOfLastLabelValue:  "a",
		},
		{
			Hash: 1,
			FirstCharacterOfFirstLabelName: "b",
			LabelMatterLength:              1,
			LastCharacterOfLastLabelValue:  "a",
		},
		{
			Hash: 1,
			FirstCharacterOfFirstLabelName: "b",
			LabelMatterLength:              1,
			LastCharacterOfLastLabelValue:  "b",
		},
	}
	for i := range fingerprints {
		if i == 0 {
			continue
		}

		if !fingerprints[i-1].Less(fingerprints[i]) {
			t.Errorf("%d expected %s < %s", i, fingerprints[i-1], fingerprints[i])
		}
	}
}

func BenchmarkFingerprinting(b *testing.B) {
	b.StopTimer()
	fps := []*Fingerprint{
		{
			Hash: 0,
			FirstCharacterOfFirstLabelName: "a",
			LabelMatterLength:              2,
			LastCharacterOfLastLabelValue:  "z",
		},
		{
			Hash: 0,
			FirstCharacterOfFirstLabelName: "a",
			LabelMatterLength:              2,
			LastCharacterOfLastLabelValue:  "z",
		},
	}
	for i := 0; i < 10; i++ {
		fps[0].Less(fps[1])
	}
	b.Logf("N: %v", b.N)
	b.StartTimer()

	var pre runtime.MemStats
	runtime.ReadMemStats(&pre)

	for i := 0; i < b.N; i++ {
		fps[0].Less(fps[1])
	}

	var post runtime.MemStats
	runtime.ReadMemStats(&post)

	b.Logf("allocs: %d items: ", post.TotalAlloc-pre.TotalAlloc)
}
