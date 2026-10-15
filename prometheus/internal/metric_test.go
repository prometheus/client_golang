// Copyright The Prometheus Authors
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

package internal

import (
	"sort"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestIsLabelPairsSorted(t *testing.T) {
	name := func(s string) *string { return &s }

	cases := []struct {
		name     string
		pairs    []*dto.LabelPair
		expected bool
	}{
		{
			name:     "empty",
			pairs:    nil,
			expected: true,
		},
		{
			name:     "single",
			pairs:    []*dto.LabelPair{{Name: name("a")}},
			expected: true,
		},
		{
			name: "sorted",
			pairs: []*dto.LabelPair{
				{Name: name("a")},
				{Name: name("b")},
				{Name: name("c")},
			},
			expected: true,
		},
		{
			name: "duplicates",
			pairs: []*dto.LabelPair{
				{Name: name("a")},
				{Name: name("a")},
			},
			expected: true,
		},
		{
			name: "unsorted",
			pairs: []*dto.LabelPair{
				{Name: name("b")},
				{Name: name("a")},
			},
			expected: false,
		},
		{
			name: "unsorted at end",
			pairs: []*dto.LabelPair{
				{Name: name("a")},
				{Name: name("c")},
				{Name: name("b")},
			},
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsLabelPairsSorted(tc.pairs)
			want := sort.IsSorted(LabelPairSorter(tc.pairs))
			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
			if got != want {
				t.Fatalf("IsLabelPairsSorted (%v) does not match sort.IsSorted (%v)", got, want)
			}
		})
	}
}
