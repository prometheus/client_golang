// Copyright 2025 The Prometheus Authors
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

package fastdto

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

func BenchmarkToDTOLabelPairs(b *testing.B) {
	test := []LabelPair{
		{"foo", "bar"},
		{"foo2", "bar2"},
		{"foo3", "bar3"},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ToDTOLabelPair(test)
	}
}

func TestLabelPairSorter(t *testing.T) {
	test := []LabelPair{
		{"foo3", "bar3"},
		{"foo", "bar"},
		{"foo2", "bar2"},
	}
	sort.Sort(LabelPairSorter(test))

	expected := []LabelPair{
		{"foo", "bar"},
		{"foo2", "bar2"},
		{"foo3", "bar3"},
	}
	if diff := cmp.Diff(test, expected); diff != "" {
		t.Fatal(diff)
	}
}

func TestToDTOLabelPair(t *testing.T) {
	test := []LabelPair{
		{"foo", "bar"},
		{"foo2", "bar2"},
		{"foo3", "bar3"},
	}
	expected := []*dto.LabelPair{
		{Name: proto.String("foo"), Value: proto.String("bar")},
		{Name: proto.String("foo2"), Value: proto.String("bar2")},
		{Name: proto.String("foo3"), Value: proto.String("bar3")},
	}
	if diff := cmp.Diff(ToDTOLabelPair(test), expected, cmpopts.IgnoreUnexported(dto.LabelPair{})); diff != "" {
		t.Fatal(diff)
	}
}
